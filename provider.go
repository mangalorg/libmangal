package libmangal

import (
	"bytes"
	"fmt"
	"github.com/metafates/libmangal/vm"
	"github.com/yuin/gluamapper"
	lua "github.com/yuin/gopher-lua"
	"path/filepath"

	"gopkg.in/yaml.v3"
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

	for _, load := range []func() error{
		provider.loadInfo,
		provider.loadFunctions,
	} {
		if err := load(); err != nil {
			return nil, err
		}
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
	file, err := p.client.fs.Open(p.path)
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

func (p *Provider) loadFunctions() error {
	state := vm.NewState(vm.Options{
		HTTPClient: p.client.httpClient,
		FS:         p.client.fs,
	})
	defer func() {
		p.state = state
	}()

	file, err := p.client.fs.Open(p.path)
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

	var (
		hasSearch,
		hasChapters bool
	)

	p.searchMangas, hasSearch = state.GetGlobal(searchMangas).(*lua.LFunction)
	p.mangaChapters, hasChapters = state.GetGlobal(mangaChapters).(*lua.LFunction)
	p.chapterPages, _ = state.GetGlobal(chapterPages).(*lua.LFunction)

	// validate
	// at least one of searchMangas or mangaChapters must be defined
	// chapterPages is optional
	if !(hasSearch || hasChapters) {
		// TODO: add more descriptive error
		return fmt.Errorf("missing function")
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

		if page.Url == "" {
			return nil, fmt.Errorf("page url must be non empty")
		}

		pages[i] = &page
	}

	return pages, nil
}
