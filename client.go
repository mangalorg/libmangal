package libmangal

import (
	"context"
	"fmt"

	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
)

// NewClient creates a new client from ProviderLoader.
// ClientOptions must be non-nil. Use DefaultClientOptions for defaults.
// It will validate ProviderLoader.Info and load the provider.
func NewClient(
	ctx context.Context,
	loader ProviderLoader,
	options ClientOptions,
) (*Client, error) {
	providerInfo := loader.Info()

	if err := providerInfo.Validate(); err != nil {
		return nil, err
	}

	provider, err := loader.Load(ctx)
	if err != nil {
		return nil, err
	}

	logger := NewLogger()
	logger.SetPrefix(providerInfo.Name)
	provider.SetLogger(logger)

	return &Client{
		provider: provider,
		options:  options,
		logger:   logger,
	}, nil
}

// Client is the wrapper around Provider with the extended functionality.
// It's the core of the libmangal
type Client struct {
	provider Provider
	options  ClientOptions
	logger   *Logger
}

func (c *Client) FS() afero.Fs {
	return c.options.FS
}

func (c *Client) Anilist() *Anilist {
	return c.options.Anilist
}

func (c *Client) Logger() *Logger {
	return c.logger
}

func (c *Client) Close() error {
	return c.provider.Close()
}

// SearchMangas searches for mangas with the given query
func (c *Client) SearchMangas(ctx context.Context, query string) ([]Manga, error) {
	return c.provider.SearchMangas(ctx, query)
}

// MangaVolumes gets chapters of the given manga
func (c *Client) MangaVolumes(ctx context.Context, manga Manga) ([]Volume, error) {
	return c.provider.MangaVolumes(ctx, manga)
}

// VolumeChapters gets chapters of the given manga
func (c *Client) VolumeChapters(ctx context.Context, volume Volume) ([]Chapter, error) {
	return c.provider.VolumeChapters(ctx, volume)
}

// ChapterPages gets pages of the given chapter
func (c *Client) ChapterPages(ctx context.Context, chapter Chapter) ([]Page, error) {
	return c.provider.ChapterPages(ctx, chapter)
}

func (c *Client) String() string {
	return c.provider.Info().Name
}

// Info returns info about provider
func (c *Client) Info() ProviderInfo {
	return c.provider.Info()
}

// DownloadChapter downloads and saves chapter to the specified
// directory in the given format.
//
// It will return resulting chapter path joined with DownloadOptions.Directory
func (c *Client) DownloadChapter(
	ctx context.Context,
	chapter Chapter,
	options DownloadOptions,
) (string, error) {
	c.logger.Log(fmt.Sprintf("Downloading chapter %q as %s", chapter, options.Format))

	tmpClient := Client{
		provider: c.provider,
		options:  c.options,
		logger:   c.logger,
	}

	tmpClient.options.FS = afero.NewMemMapFs()

	path, err := tmpClient.downloadChapterWithMetadata(ctx, chapter, options, func(path string) (bool, error) {
		return afero.Exists(c.options.FS, path)
	})
	if err != nil {
		return "", err
	}

	if err := mergeDirectories(
		c.FS(), options.Directory,
		tmpClient.FS(), options.Directory,
	); err != nil {
		return "", err
	}

	if options.ReadAfter {
		return path, c.ReadChapter(ctx, path, chapter, options.ReadOptions)
	}

	return path, nil
}

// DownloadPagesInBatch downloads multiple pages in batch
// by calling DownloadPage for each page in a separate goroutines.
// If any of the pages fails to download it will stop downloading other pages
// and return error immediately
func (c *Client) DownloadPagesInBatch(
	ctx context.Context,
	pages []Page,
) ([]PageWithImage, error) {
	c.logger.Log(fmt.Sprintf("Downloading %d pages", len(pages)))

	g, ctx := errgroup.WithContext(ctx)

	downloadedPages := make([]PageWithImage, len(pages))

	for i, page := range pages {
		// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		i, page := i, page
		g.Go(func() error {
			c.logger.Log(fmt.Sprintf("Page #%03d: downloading", i+1))

			downloaded, err := c.DownloadPage(ctx, page)
			if err != nil {
				return err
			}

			c.logger.Log(fmt.Sprintf("Page #%03d: done", i+1))

			downloadedPages[i] = downloaded
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return downloadedPages, nil
}

// DownloadPage downloads a page contents (image)
func (c *Client) DownloadPage(ctx context.Context, page Page) (PageWithImage, error) {
	if withImage, ok := page.(PageWithImage); ok {
		return withImage, nil
	}

	image, err := c.provider.GetPageImage(ctx, page)
	if err != nil {
		return nil, err
	}

	return &pageWithImage{
		Page:  page,
		image: image,
	}, nil
}

func (c *Client) ComputeMangaFilename(manga Manga) string {
	return c.options.MangaNameTemplate(c.String(), manga)
}

func (c *Client) ComputeVolumeFilename(volume Volume) string {
	return c.options.VolumeNameTemplate(c.String(), volume)
}

func (c *Client) ComputeChapterFilename(chapter Chapter, format Format) string {
	return c.options.ChapterNameTemplate(c.String(), chapter) + format.Extension()
}
