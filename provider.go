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
type ProviderLoader[M Manga, V Volume, C Chapter, P Page] interface {
	// Info information about Provider
	Info() ProviderInfo

	// Load loads the Provider
	Load(ctx context.Context) (Provider[M, V, C, P], error)
}

// Provider exposes methods for searching mangas, getting chapters, pages and images
type Provider[M Manga, V Volume, C Chapter, P Page] interface {
	// Info information about Provider
	Info() ProviderInfo

	// SearchMangas searches for mangas with the given query.
	// Implementation should utilize given LogFunc
	SearchMangas(
		ctx context.Context,
		log LogFunc,
		query string,
	) ([]M, error)

	// MangaVolumes gets volumes of the manga
	// Implementation should utilize given LogFunc
	MangaVolumes(
		ctx context.Context,
		log LogFunc,
		manga M,
	) ([]V, error)

	// VolumeChapters gets chapters of the given volume.
	// Implementation should utilize given LogFunc
	VolumeChapters(
		ctx context.Context,
		log LogFunc,
		volume V,
	) ([]C, error)

	// ChapterPages gets pages of the given chapter.
	// Implementation should utilize given LogFunc
	ChapterPages(
		ctx context.Context,
		log LogFunc,
		chapter C,
	) ([]P, error)

	// GetPageImage gets raw image contents of the given page.
	// Implementation should utilize given LogFunc
	GetPageImage(
		ctx context.Context,
		log LogFunc,
		page P,
	) (io.Reader, error)
}

// LogFunc is the function used for tracking progress of various operations
type LogFunc = func(msg string)
