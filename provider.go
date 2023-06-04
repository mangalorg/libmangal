package libmangal

import (
	"bytes"
	"context"
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

	state := vm.NewState(vm.Options{
		HTTPClient: p.client.options.HTTPClient,
		HTTPStore:  p.httpStore,
		FS:         p.client.options.FS,
	})
	defer func() {
		p.state = state
	}()

	state.SetContext(ctx)
	lfunc, err := state.Load(bytes.NewReader(p.rawScript), p.info.Name)
	if err != nil {
		return err
	}

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

	var ok bool
	p.searchMangas, ok = state.GetGlobal(searchMangas).(*lua.LFunction)
	if !ok {
		return fmt.Errorf("missing function %q", searchMangas)
	}

	p.mangaChapters, ok = state.GetGlobal(mangaChapters).(*lua.LFunction)
	if !ok {
		return fmt.Errorf("missing function %q", mangaChapters)
	}

	p.chapterPages, ok = state.GetGlobal(chapterPages).(*lua.LFunction)
	if !ok {
		return fmt.Errorf("missing function %q", chapterPages)
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
	values, err := p.evalFunction(ctx, p.searchMangas, lua.LString(query))
	if err != nil {
		return nil, err
	}

	var mangas = make([]*Manga, len(values))
	for i, value := range values {
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

	return mangas, nil
}

func (p *Provider) MangaChapters(
	ctx context.Context,
	manga *Manga,
) ([]*Chapter, error) {
	values, err := p.evalFunction(ctx, p.mangaChapters, manga.table)
	if err != nil {
		return nil, err
	}

	var chapters = make([]*Chapter, len(values))
	for i, value := range values {
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

	return chapters, nil
}

func (p *Provider) ChapterPages(
	ctx context.Context,
	chapter *Chapter,
) ([]*Page, error) {
	values, err := p.evalFunction(ctx, p.chapterPages, chapter.table)
	if err != nil {
		return nil, err
	}

	var pages = make([]*Page, len(values))
	for i, value := range values {
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

	return pages, nil
}

func (p *Provider) DownloadChapter(
	ctx context.Context,
	chapter *Chapter,
	dir string,
	options DownloadOptions,
) (path string, err error) {
	if !options.Format.IsAFormat() {
		return "", fmt.Errorf("unsupported format")
	}

	if options.CreateMangaDir {
		path = filepath.Join(dir, p.client.options.MangaNameTemplate(chapter.manga.NameData()))
		err = p.client.options.FS.MkdirAll(path, 0755)
		if err != nil {
			return "", err
		}
	} else {
		path = dir
	}

	path = filepath.Join(path, p.client.options.ChapterNameTemplate(chapter.NameData()))

	var isDir bool
	switch options.Format {
	case FormatPDF:
		path += ".pdf"
	case FormatCBZ:
		path += ".cbz"
	case FormatImages:
		isDir = true
	}

	exists, err := afero.Exists(p.client.options.FS, path)
	if err != nil {
		return path, nil
	}

	if exists {
		if options.SkipIfExists {
			return path, nil
		}

		if isDir {
			err = p.client.options.FS.RemoveAll(path)
		} else {
			err = p.client.options.FS.Remove(path)
		}

		if err != nil {
			return "", err
		}
	}

	pages, err := p.ChapterPages(ctx, chapter)
	if err != nil {
		return "", err
	}

	downloadedPages, err := p.downloadPages(ctx, pages)
	if err != nil {
		return "", err
	}

	switch options.Format {
	case FormatPDF:
		err = p.savePDF(downloadedPages, path)
	case FormatCBZ:
		err = p.saveCBZ(downloadedPages, path)
	case FormatImages:
		err = p.saveImages(downloadedPages, path)
	}

	return path, err
}

func (p *Provider) downloadPages(ctx context.Context, pages []*Page) ([]*downloadedPage, error) {
	g, _ := errgroup.WithContext(ctx)

	var downloadedPages = make([]*downloadedPage, len(pages))

	for i, page := range pages {
		i, page := i, page
		g.Go(func() error {
			reader, err := p.pageReader(ctx, page)

			if err != nil {
				return err
			}

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
) error {
	panic("not implemented")
}

func (p *Provider) saveImages(
	pages []*downloadedPage,
	path string,
) error {
	err := p.client.options.FS.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	for i, page := range pages {
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

func (p *Provider) pageReader(ctx context.Context, page *Page) (io.Reader, error) {
	if page.Data != "" {
		return strings.NewReader(page.Data), nil
	}

	request, _ := http.NewRequestWithContext(ctx, http.MethodGet, page.Url, nil)

	if page.Headers != nil {
		for key, value := range page.Headers {
			request.Header.Set(key, value)
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

func (p *Provider) ReadChapter(ctx context.Context, chapter *Chapter, options ReadOptions) error {
	tempDir, err := afero.TempDir(p.client.options.FS, "", "libmangal")
	if err != nil {
		return err
	}

	chapterPath, err := p.DownloadChapter(ctx, chapter, tempDir, DownloadOptions{
		Format:         options.Format,
		CreateMangaDir: true,
		SkipIfExists:   true,
	})
	if err != nil {
		return err
	}

	err = open.Run(chapterPath)
	if err != nil {
		return err
	}

	// TODO: history

	return nil
}
