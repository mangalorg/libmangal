package libmangal

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
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

	searchMangas, mangaChapters, chapterPages *lua.LFunction
	state                                     *lua.LState
}

func (p *Provider) Info() ProviderInfo {
	return *p.info
}

func (p *Provider) String() string {
	return p.info.Name
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

// SearchMangas searches for mangas with the given query
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

// MangaChapters gets manga chapters
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

// ChapterPages gets chapter pages
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

// DownloadChapter downloads and saves chapter to the specified
// directory in the given format.
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

	mangaFilename, chapterFilename := p.ComputeFilenames(chapter, options.Format)

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

	err = p.client.options.FS.MkdirAll(mangaTempPath, 0755)
	if err != nil {
		return "", err
	}

	err = p.downloadChapter(ctx, chapter, chapterTempPath, options)
	if err != nil {
		return "", err
	}

	err = p.client.options.FS.MkdirAll(mangaPath, 0755)
	if err != nil {
		return "", err
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
		manga, err := p.client.MakeMangaWithAnilist(ctx, chapter.manga)
		if err != nil {
			return "", err
		}

		seriesJson := manga.SeriesJson()
		seriesJsonPath := filepath.Join(mangaPath, seriesJsonFilename)

		marshalled, err := json.Marshal(seriesJson)
		if err != nil {
			return "", err
		}

		err = afero.WriteFile(p.client.options.FS, seriesJsonPath, marshalled, 0644)
		if err != nil {
			return "", err
		}
	}

	if options.DownloadMangaCover {
		err = p.downloadCoverIfNotExists(
			ctx,
			chapter.manga,
			mangaPath,
		)

		if err != nil {
			return "", err
		}
	}

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

// downloadCoverIfNotExists will download manga chapter
// and save it as a cover.jpg in the specified directory.
// It will also try to convert Manga to MangaWithAnilist
// to obtain a better cover image. If it fails to do so,
// then it will use Manga.CoverUrl.
func (p *Provider) downloadCoverIfNotExists(
	ctx context.Context,
	manga *Manga,
	dir string,
) error {
	const coverFilename = "cover.jpg"

	exists, err := afero.Exists(p.client.options.FS, filepath.Join(dir, coverFilename))
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	var coverUrl string

	if manga, err := p.client.MakeMangaWithAnilist(ctx, manga); err != nil {
		for _, url := range []string{
			manga.CoverImage.ExtraLarge,
			// no need to check `Large` cover, see `ExtraLarge` description
			manga.CoverImage.Medium,
			manga.CoverUrl,
		} {
			if url != "" {
				coverUrl = url
				break
			}
		}
	} else {
		coverUrl = manga.CoverUrl
	}

	if coverUrl == "" {
		return fmt.Errorf("cover url is empty")
	}

	p.client.options.Log("downloading cover")
	cover := Page{
		Url: coverUrl,
	}

	downloaded, err := p.DownloadPage(ctx, &cover)
	if err != nil {
		return err
	}

	return afero.WriteReader(
		p.client.options.FS,
		filepath.Join(dir, coverFilename),
		downloaded,
	)
}

// downloadChapter is a helper function for DownloadChapter
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

	downloadedPages, err := p.DownloadPagesInBatch(ctx, pages)
	if err != nil {
		return err
	}

	switch options.Format {
	case FormatPDF:
		err = p.SavePDF(downloadedPages, path)
	case FormatCBZ:
		var comicInfo *ComicInfoXml
		if options.WriteComicInfoXml {
			chapter, err := p.client.MakeChapterWithAnilist(ctx, chapter)
			if err != nil {
				return err
			}

			comicInfo = chapter.ComicInfoXml(options.ComicInfoOptions)
		}

		err = p.SaveCBZ(downloadedPages, path, comicInfo)
	case FormatImages:
		err = p.SaveImages(downloadedPages, path)
	}

	if err != nil {
		return err
	}

	return nil
}

// DownloadPagesInBatch downloads multiple pages in batch
// by calling DownloadPage for each page in a separate goroutines.
// If any of the pages fails to download it will stop downloading other pages
// and return error immediately
func (p *Provider) DownloadPagesInBatch(
	ctx context.Context,
	pages []*Page,
) ([]*DownloadedPage, error) {
	p.client.options.Log(fmt.Sprintf("Downloading %d pages", len(pages)))

	g, _ := errgroup.WithContext(ctx)

	downloadedPages := make([]*DownloadedPage, len(pages))

	for i, page := range pages {
		i, page := i, page
		g.Go(func() error {
			p.client.options.Log(fmt.Sprintf("Page #%03d: downloading", i+1))
			downloaded, err := p.DownloadPage(ctx, page)
			if err != nil {
				return err
			}

			p.client.options.Log(fmt.Sprintf("Page #%03d: done", i+1))

			downloadedPages[i] = downloaded

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return downloadedPages, nil
}

// SavePDF saves pages in FormatPDF
func (p *Provider) SavePDF(
	pages []*DownloadedPage,
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

// SaveCBZ saves pages in FormatCBZ
func (p *Provider) SaveCBZ(
	pages []*DownloadedPage,
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
			Name:   fmt.Sprintf("%04d%s", i+1, page.Extension),
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

// SaveImages saves pages in FormatImages
func (p *Provider) SaveImages(
	pages []*DownloadedPage,
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
		file, err = p.client.options.FS.Create(filepath.Join(path, fmt.Sprintf("%04d%s", i+1, page.Extension)))
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

// DownloadPage downloads a page contents (image)
func (p *Provider) DownloadPage(ctx context.Context, page *Page) (*DownloadedPage, error) {
	if page.Data != "" {
		return &DownloadedPage{
			Page:   page,
			Reader: strings.NewReader(page.Data),
		}, nil
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

	return &DownloadedPage{
		Page:   page,
		Reader: bytes.NewReader(buffer),
	}, nil
}

// ReadChapter downloads chapter to the temp directory and opens it with the
// os default app for resulting mimetype.
// E.g. `xdg-open` for Linux.
//
// Note: works only for afero.OsFs
func (p *Provider) ReadChapter(ctx context.Context, chapter *Chapter, options *ReadOptions) error {
	if p.client.options.FS.Name() != "OsFs" {
		return fmt.Errorf("only OsFs is supported for reading")
	}

	p.client.options.Log(fmt.Sprintf("Reading chapter %q as %s", chapter.Title, options.Format))

	var chapterPath string
	if options.MangasLibraryPath != "" {
		path, ok, err := p.IsChapterDownloaded(chapter, options.MangasLibraryPath, options.Format)
		if err != nil {
			return err
		}

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

		chapterPath, err = p.DownloadChapter(
			ctx,
			chapter,
			tempDir,
			DefaultDownloadOptions(),
		)
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

// IsChapterDownloaded checks if chapter is downloaded.
// It will simply check if path dir/manga/chapter exists
func (p *Provider) IsChapterDownloaded(
	chapter *Chapter,
	dir string,
	format Format,
) (path string, ok bool, err error) {
	mangaFilename, chapterFilename := p.ComputeFilenames(chapter, format)

	path = filepath.Join(dir, mangaFilename, chapterFilename)

	exists, err := afero.Exists(p.client.options.FS, path)
	if err != nil {
		return "", false, err
	}

	return path, exists, nil
}

// ComputeFilenames will apply name templates for chapter and manga
// and return resulting strings.
func (p *Provider) ComputeFilenames(
	chapter *Chapter,
	format Format,
) (mangaFilename, chapterFilename string) {
	mangaFilename = p.client.options.MangaNameTemplate(
		p.String(),
		chapter.manga.NameData(),
	)

	chapterFilename = p.client.options.ChapterNameTemplate(
		p.String(),
		chapter.NameData(),
	)

	extension, ok := FormatExtensions[format]
	if ok {
		chapterFilename += extension
	}

	return mangaFilename, chapterFilename
}
