package libmangal

import (
	"bytes"
	"context"
	"fmt"
	"github.com/mangalorg/libmangal/vm"
	"github.com/pkg/errors"
	lua "github.com/yuin/gopher-lua"
	"golang.org/x/mod/semver"
	"gopkg.in/yaml.v3"
)

type ProviderInfo struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version"`
}

func (p *ProviderInfo) validate() error {
	if p.Name == "" {
		return fmt.Errorf("name must be set")
	}

	if !semver.IsValid(p.Version) {
		return fmt.Errorf("invalid semver %q", p.Version)
	}

	return nil
}

type ProviderHandle struct {
	rawScript []byte
	client    *Client
	info      *ProviderInfo
}

func extractInfo(script []byte) (*ProviderInfo, error) {
	var (
		infoLines  [][]byte
		infoPrefix = []byte("--|")
	)

	for _, line := range bytes.Split(script, []byte("\n")) {
		if bytes.HasPrefix(line, infoPrefix) {
			infoLines = append(infoLines, bytes.TrimPrefix(line, infoPrefix))
		}
	}

	info := &ProviderInfo{}
	err := yaml.Unmarshal(bytes.Join(infoLines, []byte("\n")), info)

	if err != nil {
		return nil, err
	}

	if err := info.validate(); err != nil {
		return nil, errors.Wrap(err, "info")
	}

	return info, nil
}

func (p ProviderHandle) Info() ProviderInfo {
	return *p.info
}

func (p ProviderHandle) LoadProvider(ctx context.Context, options *ProviderLoadOptions) (*Provider, error) {
	provider := &Provider{
		client:    p.client,
		info:      p.info,
		rawScript: p.rawScript,
	}

	p.client.options.Log(fmt.Sprintf("Compiling provider %q", p.info.Name))
	state := vm.NewState(&vm.Options{
		HTTPClient: p.client.options.HTTPClient,
		HTTPStore:  options.HTTPStore,
		FS:         p.client.options.FS,
	})
	provider.state = state

	state.SetContext(ctx)
	lfunc, err := state.Load(bytes.NewReader(p.rawScript), p.info.Name)
	if err != nil {
		return nil, err
	}

	p.client.options.Log(fmt.Sprintf("Initializing provider %q", p.info.Name))
	if err = state.CallByParam(lua.P{
		Fn:      lfunc,
		NRet:    0,
		Protect: true,
	}); err != nil {
		return nil, err
	}

	// TODO: move to a separate file or module to use in other places
	// e.g. templates
	const (
		searchMangas  = "SearchMangas"
		mangaChapters = "MangaChapters"
		chapterPages  = "ChapterPages"
	)

	for name, fn := range map[string]**lua.LFunction{
		searchMangas:  &provider.searchMangas,
		mangaChapters: &provider.mangaChapters,
		chapterPages:  &provider.chapterPages,
	} {
		p.client.options.Log(fmt.Sprintf("Loading function %q", name))

		var ok bool
		*fn, ok = state.GetGlobal(name).(*lua.LFunction)
		if !ok {
			return nil, fmt.Errorf("missing function %q", name)
		}
	}

	return provider, nil
}
