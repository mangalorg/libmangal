package libmangal

import (
	"bytes"
	"fmt"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
	"github.com/pkg/errors"
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
		return fmt.Errorf("title must be set")
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

func (p ProviderHandle) LoadProvider(httpStore gokv.Store) (*Provider, error) {
	if httpStore == nil {
		httpStore = syncmap.NewStore(syncmap.DefaultOptions)
	}

	provider := &Provider{
		client:    p.client,
		httpStore: httpStore,
		info:      p.info,
		rawScript: p.rawScript,
	}

	return provider, nil
}
