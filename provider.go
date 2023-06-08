package libmangal

import (
	"context"
	"errors"
	"fmt"
	"golang.org/x/mod/semver"
	"io"
)

// ProviderInfo is the passport of the provider
type ProviderInfo struct {
	// Name is the non-empty name of the provider
	Name string

	// Version is valid semantic version of the provider.
	// Must start with the "v". E.g. "v0.1.0".
	// See https://semver.org/
	Version string

	// Description of the provider. May be empty.
	Description string
}

// Validate checks if the ProviderInfo is valid.
// This means that ProviderInfo.Name is non-empty
// and ProviderInfo.Version is a valid semver
func (p ProviderInfo) Validate() error {
	if p.Name == "" {
		return errors.New("name must be non-empty")
	}

	if !semver.IsValid(p.Version) {
		return fmt.Errorf("invalid semver: %s", p.Version)
	}

	return nil
}

// ProviderLoader gives information about provider without loading it first.
type ProviderLoader interface {
	// Info information about Provider
	Info() ProviderInfo

	// Load loads the Provider
	Load(ctx context.Context) (Provider, error)
}

// Provider exposes methods for searching mangas, getting chapters, pages and images
type Provider interface {
	// Info information about Provider
	Info() ProviderInfo

	// SearchMangas searches for mangas with the given query.
	// Implementation should utilize given LogFunc
	SearchMangas(
		ctx context.Context,
		log LogFunc,
		query string,
	) ([]Manga, error)

	// MangaVolumes gets volumes of the manga
	// Implementation should utilize given LogFunc
	MangaVolumes(
		ctx context.Context,
		log LogFunc,
		manga Manga,
	) ([]Volume, error)

	// VolumeChapters gets chapters of the given volume.
	// Implementation should utilize given LogFunc
	VolumeChapters(
		ctx context.Context,
		log LogFunc,
		volume Volume,
	) ([]Chapter, error)

	// ChapterPages gets pages of the given chapter.
	// Implementation should utilize given LogFunc
	ChapterPages(
		ctx context.Context,
		log LogFunc,
		chapter Chapter,
	) ([]Page, error)

	// GetPageImage gets raw image contents of the given page.
	// Implementation should utilize given LogFunc
	GetPageImage(
		ctx context.Context,
		log LogFunc,
		page Page,
	) (io.Reader, error)
}

// LogFunc is the function used for tracking progress of various operations
type LogFunc = func(msg string)
