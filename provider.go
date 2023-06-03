package libmangal

import (
	"bytes"
	"fmt"
	"github.com/metafates/libmangal/vm"
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

func (p *Provider) SearchMangas(query string) ([]*Manga, error) {
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

func (p *Provider) MangaChapters(manga *Manga) ([]*Chapter, error) {
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

func (p *Provider) ChapterPages(chapter *Chapter) ([]*Page, error) {
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

func (p *Provider) DownloadChapter(chapter *Chapter, dir string, options DownloadOptions) error {
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

	switch options.Format {
	case FormatPDF:
		err = p.downloadChapterPDF(chapter, path)
	case FormatCBZ:
		err = p.downloadChapterCBZ(chapter, path)
	case FormatImages:
		err = p.downloadChapterImages(chapter, path)
	}

	return err
}

func (p *Provider) downloadChapterPDF(chapter *Chapter, path string) error {
	panic("not implemented")
}

func (p *Provider) downloadChapterCBZ(chapter *Chapter, path string) error {
	panic("not implemented")
}

func (p *Provider) downloadChapterImages(chapter *Chapter, path string) error {
	pages, err := p.ChapterPages(chapter)
	if err != nil {
		return err
	}

	g, _ := errgroup.WithContext(p.client.context)

	var downloadedPages = make([]downloadedPage, len(pages))

	for i, page := range pages {
		i, page := i, page
		g.Go(func() error {
			var reader io.Reader
			reader, err = p.pageReader(page)

			if err != nil {
				return err
			}

			downloadedPages[i] = downloadedPage{
				Page:   page,
				Reader: reader,
			}

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return err
	}

	// create directory
	err = p.client.options.FS.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	for i, page := range downloadedPages {
		if page.Reader == nil {
			return fmt.Errorf("reader %d is nil", i)
		}

		var file afero.File
		file, err = p.client.options.FS.Create(filepath.Join(path, fmt.Sprintf("%03d.%s", i+1, page.Extension)))
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

func (p *Provider) pageReader(page *Page) (io.Reader, error) {
	if page.Data != "" {
		return strings.NewReader(page.Data), nil
	}

	request, _ := http.NewRequestWithContext(p.client.context, http.MethodGet, page.Url, nil)

	if page.Headers != nil {
		for key, value := range page.Headers {
			request.Header.Set(key, value)
		}
	}

	response, err := p.client.options.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", response.StatusCode)
	}

	return response.Body, nil
}
