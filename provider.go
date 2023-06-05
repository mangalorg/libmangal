package libmangal

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/mangalorg/libmangal/vm"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/philippgille/gokv"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/afero"
	"github.com/yuin/gluamapper"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

type Provider struct {
	rawScript []byte
	info      *ProviderInfo
	client    *Client

	httpStore gokv.Store

	searchMangas, mangaChapters, chapterPages *lua.LFunction
	state                                     *lua.LState
}

func (p *Provider) Info() ProviderInfo {
	return *p.info
}

func (p *Provider) Load(ctx context.Context) error {
	if p.state != nil {
		return fmt.Errorf("already loaded")
	}

	p.client.options.Log(fmt.Sprintf("Compiling provider %q", p.info.Name))
	state := vm.NewState(&vm.Options{
		HTTPClient: p.client.options.HTTPClient,
		HTTPStore:  p.httpStore,
		FS:         p.client.options.FS,
	})
	p.state = state

	state.SetContext(ctx)
	lfunc, err := state.Load(bytes.NewReader(p.rawScript), p.info.Name)
	if err != nil {
		return err
	}

	p.client.options.Log(fmt.Sprintf("Initializing provider %q", p.info.Name))
	if err = state.CallByParam(lua.P{
		Fn:      lfunc,
		NRet:    0,
		Protect: true,
	}); err != nil {
		return err
	}

	// TODO: move to a separate file or module to use in other places
	// e.g. templates
	const (
		searchMangas  = "SearchMangas"
		mangaChapters = "MangaChapters"
		chapterPages  = "ChapterPages"
	)

	for name, fn := range map[string]**lua.LFunction{
		searchMangas:  &p.searchMangas,
		mangaChapters: &p.mangaChapters,
		chapterPages:  &p.chapterPages,
	} {
		p.client.options.Log(fmt.Sprintf("Loading function %q", name))

		var ok bool
		*fn, ok = state.GetGlobal(name).(*lua.LFunction)
		if !ok {
			return fmt.Errorf("missing function %q", name)
		}
	}

	return nil
}

func (p *Provider) evalFunction(
	ctx context.Context,
	fn *lua.LFunction,
	input lua.LValue,
) (output []lua.LValue, err error) {
	p.state.SetContext(ctx)
	err = p.state.CallByParam(lua.P{
		Fn:      fn,
		NRet:    1,
		Protect: true,
	}, input)

	if err != nil {
		return nil, err
	}

	p.
		state.
		CheckTable(-1).
		ForEach(func(_, value lua.LValue) {
			output = append(output, value)
		})

	return
}

func (p *Provider) SearchMangas(
	ctx context.Context,
	query string,
) ([]*Manga, error) {
	p.client.options.Log(fmt.Sprintf("Searching mangas for %q", query))

	values, err := p.evalFunction(ctx, p.searchMangas, lua.LString(query))
	if err != nil {
		return nil, err
	}

	var mangas = make([]*Manga, len(values))
	for i, value := range values {
		p.client.options.Log(fmt.Sprintf("Parsing manga #%03d", i+1))

		table, ok := value.(*lua.LTable)
		if !ok {
			// TODO: add more descriptive message
			return nil, fmt.Errorf("table expected")
		}

		var manga Manga
		if err = gluamapper.Map(table, &manga); err != nil {
			return nil, err
		}

		if err = manga.validate(); err != nil {
			return nil, err
		}

		manga.table = table
		mangas[i] = &manga
	}

	p.client.options.Log(fmt.Sprintf("Found %d mangas", len(mangas)))
	return mangas, nil
}

func (p *Provider) MangaChapters(
	ctx context.Context,
	manga *Manga,
) ([]*Chapter, error) {
	p.client.options.Log(fmt.Sprintf("Fetching chapters for %q", manga.Title))
	values, err := p.evalFunction(ctx, p.mangaChapters, manga.table)
	if err != nil {
		return nil, err
	}

	var chapters = make([]*Chapter, len(values))
	for i, value := range values {
		p.client.options.Log(fmt.Sprintf("Parsing chapter #%04d", i))

		table, ok := value.(*lua.LTable)
		if !ok {
			// TODO: add more descriptive message
			return nil, fmt.Errorf("table expected")
		}

		var chapter Chapter
		if err = gluamapper.Map(table, &chapter); err != nil {
			return nil, err
		}

		if err = chapter.validate(); err != nil {
			return nil, err
		}

		chapter.table = table
		chapter.manga = manga

		if chapter.Number == "" {
			chapter.Number = strconv.Itoa(i + 1)
		}

		chapters[i] = &chapter
	}

	p.client.options.Log(fmt.Sprintf("Found %d chapters", len(chapters)))
	return chapters, nil
}

func (p *Provider) ChapterPages(
	ctx context.Context,
	chapter *Chapter,
) ([]*Page, error) {
	p.client.options.Log(fmt.Sprintf("Fetching pages for %q", chapter.Title))

	values, err := p.evalFunction(ctx, p.chapterPages, chapter.table)
	if err != nil {
		return nil, err
	}

	var pages = make([]*Page, len(values))
	for i, value := range values {
		p.client.options.Log(fmt.Sprintf("Parsing page #%03d", i+1))

		table, ok := value.(*lua.LTable)
		if !ok {
			// TODO: add more descriptive message
			return nil, fmt.Errorf("table expected")
		}

		var page Page
		if err = gluamapper.Map(table, &page); err != nil {
			return nil, err
		}

		if err = page.validate(); err != nil {
			return nil, err
		}

		page.chapter = chapter
		page.fillDefaults()

		pages[i] = &page
	}

	p.client.options.Log(fmt.Sprintf("Found %d pages", len(pages)))
	return pages, nil
}

func (p *Provider) DownloadChapter(
	ctx context.Context,
	chapter *Chapter,
	dir string,
	options *DownloadOptions,
) (string, error) {
	if !options.Format.IsAFormat() {
		return "", fmt.Errorf("unsupported format")
	}

	p.client.options.Log(fmt.Sprintf("Downloading chapter %q as %s", chapter.Title, options.Format.String()))

	mangaFilename, chapterFilename := p.computeFilenames(chapter, options.Format)

	var (
		mangaPath   = filepath.Join(dir, mangaFilename)
		chapterPath = filepath.Join(mangaPath, chapterFilename)
	)

	exists, err := afero.Exists(p.client.options.FS, chapterPath)
	if err != nil {
		return chapterFilename, nil
	}

	if exists && options.SkipIfExists {
		p.client.options.Log(fmt.Sprintf("Chapter %q already exists, skipping", chapter.Title))
		return chapterFilename, nil
	}

	// create a temp dir where chapter will be downloaded.
	// after successful download chapter will be moved to the original location
	tempDir, err := afero.TempDir(p.client.options.FS, "", "")
	if err != nil {
		return "", err
	}

	var (
		mangaTempPath   = filepath.Join(tempDir, mangaFilename)
		chapterTempPath = filepath.Join(mangaTempPath, chapterFilename)
	)

	if options.CreateMangaDir {
		err = p.client.options.FS.MkdirAll(mangaTempPath, 0755)
		if err != nil {
			return "", err
		}
	}

	err = p.downloadChapter(ctx, chapter, chapterTempPath, options)
	if err != nil {
		return "", err
	}

	if options.CreateMangaDir {
		err = p.client.options.FS.MkdirAll(mangaPath, 0755)
		if err != nil {
			return "", err
		}
	}

	if exists {
		p.client.options.Log(fmt.Sprintf("Chapter %q already exists, removing", chapter.Title))
		if options.Format == FormatImages {
			err = p.client.options.FS.RemoveAll(chapterPath)
		} else {
			err = p.client.options.FS.Remove(chapterPath)
		}

		if err != nil {
			return "", err
		}
	}

	if err != nil {
		return "", err
	}

	if options.WriteSeriesJson {
		// skip if series.json already exists where it needs to be
		exists, err := afero.Exists(p.client.options.FS, filepath.Join(mangaPath, seriesJsonFilename))
		if err != nil {
			return "", err
		}

		if exists {
			goto renameChapter
		}

		// check if series.json was created in the temp dir
		// generally, should be always true but worth checking
		exists, err = afero.Exists(p.client.options.FS, filepath.Join(mangaTempPath, seriesJsonFilename))
		if err != nil {
			return "", err
		}

		if !exists {
			goto renameChapter
		}

		// move series.json from the temp directory to the target
		err = p.client.options.FS.Rename(
			filepath.Join(mangaTempPath, seriesJsonFilename),
			filepath.Join(mangaPath, seriesJsonFilename),
		)

		if err != nil {
			return "", err
		}
	}

renameChapter:
	// move to the original location
	// only after everything else was successful
	err = p.client.options.FS.Rename(
		chapterTempPath,
		chapterPath,
	)
	if err != nil {
		return "", err
	}

	return chapterPath, nil
}

func (p *Provider) computeFilenames(
	chapter *Chapter,
	format Format,
) (mangaFilename, chapterFilename string) {
	mangaFilename = p.client.options.MangaNameTemplate(chapter.manga.NameData())
	chapterFilename = p.client.options.ChapterNameTemplate(chapter.NameData())

	extensions := map[Format]string{
		FormatPDF: ".pdf",
		FormatCBZ: ".cbz",
	}

	extension, ok := extensions[format]
	if ok {
		chapterFilename += extension
	}

	return mangaFilename, chapterFilename
}

func (p *Provider) downloadChapter(
	ctx context.Context,
	chapter *Chapter,
	path string,
	options *DownloadOptions,
) error {
	pages, err := p.ChapterPages(ctx, chapter)
	if err != nil {
		return err
	}

	downloadedPages, err := p.downloadPages(ctx, pages)
	if err != nil {
		return err
	}

	switch options.Format {
	case FormatPDF:
		err = p.savePDF(downloadedPages, path)
	case FormatCBZ:
		var comicInfo *ComicInfoXml
		if options.WriteComicInfoXml {
			chapter, err := p.client.MakeChapterWithAnilist(ctx, chapter)
			if err != nil {
				return err
			}

			comicInfo = chapter.ComicInfoXml(options.ComicInfoOptions)
		}

		err = p.saveCBZ(downloadedPages, path, comicInfo)
	case FormatImages:
		err = p.saveImages(downloadedPages, path)
	}

	if err != nil {
		return err
	}

	if options.WriteSeriesJson {
		manga, err := p.client.MakeMangaWithAnilist(ctx, chapter.manga)
		if err != nil {
			return err
		}

		seriesJson := manga.SeriesJson()
		seriesJsonPath := filepath.Join(filepath.Dir(path), seriesJsonFilename)

		marshalled, err := json.Marshal(seriesJson)
		if err != nil {
			return err
		}

		err = afero.WriteFile(p.client.options.FS, seriesJsonPath, marshalled, 0644)
		if err != nil {
			// TODO: make a separate error type for that
			return err
		}
	}

	return nil
}

func (p *Provider) downloadPages(
	ctx context.Context,
	pages []*Page,
) ([]*downloadedPage, error) {
	p.client.options.Log(fmt.Sprintf("Downloading %d pages", len(pages)))

	g, _ := errgroup.WithContext(ctx)

	downloadedPages := make([]*downloadedPage, len(pages))

	for i, page := range pages {
		i, page := i, page
		g.Go(func() error {
			p.client.options.Log(fmt.Sprintf("Page #%03d: downloading", i+1))
			reader, err := p.pageToReader(ctx, page)
			if err != nil {
				return err
			}

			p.client.options.Log(fmt.Sprintf("Page #%03d: done", i+1))

			downloadedPages[i] = &downloadedPage{
				Page:   page,
				Reader: reader,
			}

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return downloadedPages, nil
}

func (p *Provider) savePDF(
	pages []*downloadedPage,
	path string,
) error {
	p.client.options.Log(fmt.Sprintf("Saving %d pages as PDF", len(pages)))

	var file afero.File
	file, err := p.client.options.FS.Create(path)
	if err != nil {
		return err
	}

	defer file.Close()

	// convert to readers
	var images = make([]io.Reader, len(pages))
	for i, page := range pages {
		images[i] = page.Reader
	}

	return api.ImportImages(nil, file, images, nil, nil)
}

func (p *Provider) saveCBZ(
	pages []*downloadedPage,
	path string,
	comicInfoXml *ComicInfoXml,
) error {
	p.client.options.Log(fmt.Sprintf("Saving %d pages as CBZ", len(pages)))

	var file afero.File
	file, err := p.client.options.FS.Create(path)
	if err != nil {
		return err
	}

	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	for i, page := range pages {
		p.client.options.Log(fmt.Sprintf("Saving page #%d", i))

		if page.Reader == nil {
			// this should not occur, but just for the safety
			return fmt.Errorf("reader %d is nil", i)
		}

		var writer io.Writer
		writer, err = zipWriter.CreateHeader(&zip.FileHeader{
			Name:   fmt.Sprintf("%04d.%s", i+1, page.Extension),
			Method: zip.Store,
		})

		if err != nil {
			return err
		}

		_, err = io.Copy(writer, page.Reader)
		if err != nil {
			return err
		}
	}

	if comicInfoXml != nil {
		marshalled, err := xml.MarshalIndent(comicInfoXml, "", "  ")
		if err != nil {
			return err
		}

		writer, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:   comicInfoXmlFilename,
			Method: zip.Store,
		})
		if err != nil {
			return err
		}

		_, err = writer.Write(marshalled)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p *Provider) saveImages(
	pages []*downloadedPage,
	path string,
) error {
	p.client.options.Log(fmt.Sprintf("Saving %d pages as images dir", len(pages)))
	err := p.client.options.FS.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	for i, page := range pages {
		p.client.options.Log(fmt.Sprintf("Saving page #%d", i))

		if page.Reader == nil {
			// this should not occur, but just for the safety
			return fmt.Errorf("reader %d is nil", i)
		}

		var file afero.File
		file, err = p.client.options.FS.Create(filepath.Join(path, fmt.Sprintf("%04d.%s", i+1, page.Extension)))
		if err != nil {
			return err
		}

		_, err = io.Copy(file, page)
		if err != nil {
			return err
		}

		_ = file.Close()
	}

	return nil
}

func (p *Provider) pageToReader(ctx context.Context, page *Page) (io.Reader, error) {
	if page.Data != "" {
		return strings.NewReader(page.Data), nil
	}

	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, page.Url, nil)

	if page.Headers != nil {
		for key, value := range page.Headers {
			request.Header.Set(key, value)
		}
	}

	if page.Cookies != nil {
		for key, value := range page.Cookies {
			request.AddCookie(&http.Cookie{Name: key, Value: value})
		}
	}

	response, err := p.client.options.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	var buffer []byte

	// check content length
	if response.ContentLength > 0 {
		buffer = make([]byte, response.ContentLength)
		_, err = io.ReadFull(response.Body, buffer)
	} else {
		buffer, err = io.ReadAll(response.Body)
	}

	if err != nil {
		return nil, err
	}

	return bytes.NewReader(buffer), nil
}

func (p *Provider) ReadChapter(ctx context.Context, chapter *Chapter, options *ReadOptions) error {
	p.client.options.Log(fmt.Sprintf("Reading chapter %q as %s", chapter.Title, options.Format))

	var chapterPath string
	if options.MangasLibraryPath != "" {
		path, ok := p.IsChapterDownloaded(options.MangasLibraryPath, chapter, options.Format)
		if ok {
			p.client.options.Log(fmt.Sprintf("Chapter %q is already downloaded", chapter.Title))
			chapterPath = path
		}
	}

	if chapterPath == "" {
		p.client.options.Log(fmt.Sprintf("Creating temp dir"))
		tempDir, err := afero.TempDir(p.client.options.FS, "", "libmangal")
		if err != nil {
			return err
		}

		downloadOptions := DefaultDownloadOptions()
		chapterPath, err = p.DownloadChapter(ctx, chapter, tempDir, downloadOptions)
		if err != nil {
			return err
		}
	}

	p.client.options.Log(fmt.Sprintf("Opening chapter %q with default app", chapter.Title))
	err := open.Run(chapterPath)
	if err != nil {
		return err
	}

	// TODO: history?

	return nil
}

// IsChapterDownloaded checks if a chapter of a manga has been
// downloaded in the specified format. It does this by checking
// if the file exists in the directory specified by the dir parameter.
func (p *Provider) IsChapterDownloaded(
	dir string,
	chapter *Chapter,
	format Format,
) (downloadedPath string, isDownloaded bool) {
	mangaFilename, chapterFilename := p.computeFilenames(chapter, format)

	isExist := func(path string) bool {
		exists, err := afero.Exists(
			p.client.options.FS,
			filepath.Join(path),
		)

		return exists && err == nil
	}

	var path string

	path = filepath.Join(dir, mangaFilename, chapterFilename)
	if isExist(path) {
		return path, true
	}

	// try without manga folder
	path = filepath.Join(dir, chapterFilename)
	if isExist(path) {
		return path, true
	}

	return "", false
}
