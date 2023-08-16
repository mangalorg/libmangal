package libmangal

import (
	"context"
	"errors"
	"fmt"
	"io"

	"golang.org/x/mod/semver"
)

// ProviderInfo is the passport of the provider
type ProviderInfo struct {
	// ID is the unique identifier of the provider
	ID string `json:"id"`

	// Name is the non-empty name of the provider
	Name string `json:"name"`

	// Version is a semantic version of the provider.
	//
	// "v" prefix is not permitted.
	// E.g. "0.1.0" is valid, but "v0.1.0" is not.
	//
	// See https://semver.org/
	Version string `json:"version"`

	// Description of the provider. May be empty.
	Description string `json:"description"`

	// Website of the provider. May be empty.
	Website string `json:"website"`
}

// Validate checks if the ProviderInfo is valid.
// This means that ProviderInfo.Name is non-empty
// and ProviderInfo.Version is a valid semver
func (p ProviderInfo) Validate() error {
	if p.ID == "" {
		return errors.New("ID must be non-empty")
	}

	if p.Name == "" {
		return errors.New("name must be non-empty")
	}

	// according to the semver specification,
	// versions should not have "v" prefix. E.g. v0.1.0 isn't a valid semver,
	// however, for some bizarre reason, Go semver package requires this prefix.
	if !semver.IsValid("v" + p.Version) {
		return fmt.Errorf("invalid semver: %s", p.Version)
	}

	return nil
}

// ProviderLoader gives information about provider without loading it first.
type ProviderLoader interface {
	fmt.Stringer

	// Info information about Provider
	Info() ProviderInfo

	// Load loads the Provider
	Load(ctx context.Context) (Provider, error)
}

// Provider exposes methods for searching mangas, getting chapters, pages and images
type Provider interface {
	fmt.Stringer
	io.Closer

	// Info information about Provider
	Info() ProviderInfo

	// SetLogger sets logger to use for this provider
	SetLogger(*Logger)

	// SearchMangas searches for mangas with the given query.
	//
	// Implementation should utilize given logger
	SearchMangas(
		ctx context.Context,
		query string,
	) ([]Manga, error)

	// MangaVolumes gets volumes of the manga
	//
	// Implementation should utilize given logger
	MangaVolumes(
		ctx context.Context,
		manga Manga,
	) ([]Volume, error)

	// VolumeChapters gets chapters of the given volume.
	//
	// Implementation should utilize given logger
	VolumeChapters(
		ctx context.Context,
		volume Volume,
	) ([]Chapter, error)

	// ChapterPages gets pages of the given chapter.
	//
	// Implementation should utilize given logger
	ChapterPages(
		ctx context.Context,
		chapter Chapter,
	) ([]Page, error)

	// GetPageImage gets raw image contents of the given page.
	//
	// Implementation should utilize given loggger
	GetPageImage(
		ctx context.Context,
		page Page,
	) ([]byte, error)
}
