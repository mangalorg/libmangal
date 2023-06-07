package libmangal

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/mod/semver"
	"io"
)

type ProviderInfo struct {
	Name        string
	Version     string
	Description string
}

func (p ProviderInfo) Validate() error {
	if p.Name == "" {
		return errors.New("name must be non-empty")
	}

	if !semver.IsValid(p.Version) {
		return fmt.Errorf("invalid semver: %s", p.Version)
	}

	return nil
}

type ProviderLoader interface {
	Info() ProviderInfo
	Load(ctx context.Context) (Provider, error)
}

type Provider interface {
	Info() ProviderInfo
	SearchMangas(ctx context.Context, log LogFunc, query string) ([]Manga, error)
	MangaChapters(ctx context.Context, log LogFunc, manga Manga) ([]Chapter, error)
	ChapterPages(ctx context.Context, log LogFunc, chapter Chapter) ([]Page, error)
	GetImage(ctx context.Context, log LogFunc, page Page) (io.Reader, error)
}

type LogFunc = func(string)
