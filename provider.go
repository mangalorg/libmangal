package libmangal

import (
	"bytes"
	"context"
	"fmt"
	"github.com/mangalorg/libmangal/vm"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/spf13/afero"
	"github.com/yuin/gluamapper"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
)

type ProviderHandle struct {
	client *Client
	path   string
}

func (p ProviderHandle) Path() string {
	return p.path
}

func (p ProviderHandle) Filename() string {
	return filepath.Base(p.path)
}

func (p ProviderHandle) Provider() (*Provider, error) {
	provider := &Provider{
		path:   p.path,
		client: p.client,
	}

	if err := provider.loadInfo(); err != nil {
		return nil, err
	}

	return provider, nil
}

type ProviderInfo struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

type Provider struct {
	info   ProviderInfo
	path   string
	client *Client

	searchMangas, mangaChapters, chapterPages *lua.LFunction
	state                                     *lua.LState
}

func (p *Provider) Info() ProviderInfo {
	return p.info
}

func (p *Provider) loadInfo() error {
	file, err := p.client.options.FS.Open(p.path)
	if err != nil {
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return fmt.Errorf("not a file")
	}

	var buffer = make([]byte, stat.Size())
	_, err = file.Read(buffer)
	if err != nil {
		return err
	}

	var (
		infoLines  [][]byte
		infoPrefix = []byte("--|")
	)

	for _, line := range bytes.Split(buffer, []byte("\n")) {
		if bytes.HasPrefix(line, infoPrefix) {
			infoLines = append(infoLines, bytes.TrimPrefix(line, infoPrefix))
		}
	}

	info := ProviderInfo{}
	err = yaml.Unmarshal(bytes.Join(infoLines, []byte("\n")), &info)

	if err != nil {
		return err
	}

	p.info = info
	return nil
}

func (p *Provider) Load() error {
	if p.state != nil {
		return fmt.Errorf("already loaded")
	}

	state := vm.NewState(vm.Options{
		HTTPClient: p.client.options.HTTPClient,
		FS:         p.client.options.FS,
	})
	defer func() {
		p.state = state
	}()

	file, err := p.client.options.FS.Open(p.path)
	if err != nil {
		return err
	}

	stat, err := file.Stat()
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return fmt.Errorf("not a file")
	}

	lfunc, err := state.Load(file, filepath.Base(p.path))
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

func (p *Provider) SearchMangas(ctx context.Context, query string) ([]*Manga, error) {
	p.state.SetContext(ctx)
	err := p.state.CallByParam(lua.P{
		Fn:      p.searchMangas,
		NRet:    1,
		Protect: true,
	}, lua.LString(query))

	if err != nil {
		return nil, err
	}

	list := p.state.CheckTable(-1)

	var values []lua.LValue
	list.ForEach(func(_ lua.LValue, value lua.LValue) {
		values = append(values, value)
	})

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

		if manga.Title == "" {
			return nil, fmt.Errorf("manga title must be non empty")
		}

		manga.table = table
		mangas[i] = &manga
	}

	return mangas, nil
}

func (p *Provider) MangaChapters(ctx context.Context, manga *Manga) ([]*Chapter, error) {
	p.state.SetContext(ctx)
	err := p.state.CallByParam(lua.P{
		Fn:      p.mangaChapters,
		NRet:    1,
		Protect: true,
	}, manga.table)

	if err != nil {
		return nil, err
	}

	list := p.state.CheckTable(-1)

	var values []lua.LValue
	list.ForEach(func(_ lua.LValue, value lua.LValue) {
		values = append(values, value)
	})

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

		if chapter.Title == "" {
			return nil, fmt.Errorf("chapter title must be non empty")
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

func (p *Provider) ChapterPages(ctx context.Context, chapter *Chapter) ([]*Page, error) {
	p.state.SetContext(ctx)
	err := p.state.CallByParam(lua.P{
		Fn:      p.chapterPages,
		NRet:    1,
		Protect: true,
	}, chapter.table)

	if err != nil {
		return nil, err
	}

	list := p.state.CheckTable(-1)

	var values []lua.LValue
	list.ForEach(func(_ lua.LValue, value lua.LValue) {
		values = append(values, value)
	})

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

		if page.Url == "" && page.Data == "" {
			return nil, fmt.Errorf("either page url or data must be non empty")
		}

		page.chapter = chapter
		page.fillDefaults()

		pages[i] = &page
	}

	return pages, nil
}

type DownloadOptions struct {
	Format         Format
	CreateMangaDir bool
	SkipIfExists   bool
}

func (p *Provider) DownloadChapter(
	ctx context.Context,
	chapter *Chapter,
	dir string,
	options DownloadOptions,
) error {
	if !options.Format.IsAFormat() {
		return fmt.Errorf("unsupported format")
	}

	var path string

	if options.CreateMangaDir {
		path = filepath.Join(dir, p.client.options.MangaNameTemplate(chapter.manga.nameData()))
		err := p.client.options.FS.MkdirAll(path, 0755)
		if err != nil {
			return err
		}
	} else {
		path = dir
	}

	path = filepath.Join(path, p.client.options.ChapterNameTemplate(chapter.nameData()))

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
		return err
	}

	if exists {
		if options.SkipIfExists {
			return nil
		}

		if isDir {
			err = p.client.options.FS.RemoveAll(path)
		} else {
			err = p.client.options.FS.Remove(path)
		}

		if err != nil {
			return err
		}
	}

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
		err = p.saveCBZ(downloadedPages, path)
	case FormatImages:
		err = p.saveImages(downloadedPages, path)
	}

	return err
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
