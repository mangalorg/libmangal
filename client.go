package libmangal

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"io"
	"path/filepath"
)

// NewClient creates a new client from ProviderLoader.
// ClientOptions must be non-nil. Use DefaultClientOptions for defaults.
// It will validate ProviderLoader.Info and load the provider.
func NewClient(
	ctx context.Context,
	loader ProviderLoader,
	options ClientOptions,
) (Client, error) {
	if err := loader.Info().Validate(); err != nil {
		return Client{}, err
	}

	provider, err := loader.Load(ctx)
	if err != nil {
		return Client{}, err
	}

	return Client{
		provider: provider,
		Options:  options,
	}, nil
}

// Client is the wrapper around Provider with the extended functionality.
// It's the core of the libmangal
type Client struct {
	provider Provider
	Options  ClientOptions
}

// SearchMangas searches for mangas with the given query
func (c Client) SearchMangas(ctx context.Context, query string) ([]Manga, error) {
	return c.provider.SearchMangas(ctx, c.Options.Log, query)
}

// MangaVolumes gets chapters of the given manga
func (c Client) MangaVolumes(ctx context.Context, manga Manga) ([]Volume, error) {
	return c.provider.MangaVolumes(ctx, c.Options.Log, manga)
}

// VolumeChapters gets chapters of the given manga
func (c Client) VolumeChapters(ctx context.Context, volume Volume) ([]Chapter, error) {
	return c.provider.VolumeChapters(ctx, c.Options.Log, volume)
}

// ChapterPages gets pages of the given chapter
func (c Client) ChapterPages(ctx context.Context, chapter Chapter) ([]Page, error) {
	return c.provider.ChapterPages(ctx, c.Options.Log, chapter)
}

func (c Client) String() string {
	return c.provider.Info().Name
}

// Info returns info about provider
func (c Client) Info() ProviderInfo {
	return c.provider.Info()
}

// DownloadChapter downloads and saves chapter to the specified
// directory in the given format.
//
// It will return resulting chapter path joined with DownloadOptions.Directory
func (c Client) DownloadChapter(
	ctx context.Context,
	chapter Chapter,
	options DownloadOptions,
) (string, error) {
	c.Options.Log(fmt.Sprintf("Downloading chapter %q as %s", chapter, options.Format))

	filenames := c.ComputeFilenames(chapter, options.Format)

	if options.CreateMangaDir {
		options.Directory = filepath.Join(options.Directory, filenames.Manga)
	}

	if options.CreateVolumeDir {
		options.Directory = filepath.Join(options.Directory, filenames.Volume)
	}

	err := c.Options.FS.MkdirAll(options.Directory, modeDir)
	if err != nil {
		return "", err
	}

	chapterPath := filepath.Join(options.Directory, filenames.Chapter)

	chapterExists, err := afero.Exists(c.Options.FS, chapterPath)
	if err != nil {
		return "", err
	}

	if chapterExists && options.SkipIfExists {
		c.Options.Log(fmt.Sprintf("Chapter %q already exists, skipping", chapter))

		if options.ReadAfter {
			return chapterPath, c.readChapter(ctx, chapterPath, chapter, options.ReadIncognito)
		}

		return chapterPath, nil
	}

	// create a temp dir where chapter will be downloaded.
	// after successful download chapter will be moved to the original location
	tempDir, err := afero.TempDir(c.Options.FS, "", "")
	if err != nil {
		return "", err
	}

	chapterTempPath := filepath.Join(tempDir, filenames.Chapter)

	err = c.downloadChapter(ctx, chapter, chapterTempPath, options)
	if err != nil {
		return "", err
	}

	if chapterExists {
		err := c.removeChapter(chapterPath)
		if err != nil {
			return "", err
		}
	}

	if options.WriteSeriesJson {
		err := c.writeSeriesJSON(ctx, chapter, options.Directory)
		if err != nil && options.Strict {
			return "", MetadataError{err}
		}
	}

	if options.DownloadMangaCover {
		err := c.downloadCover(ctx, chapter, options.Directory)
		if err != nil && options.Strict {
			return "", MetadataError{err}
		}
	}

	if options.DownloadMangaBanner {
		err := c.downloadBanner(ctx, chapter, options.Directory)
		if err != nil && options.Strict {
			return "", MetadataError{err}
		}
	}

	// move to the original location
	// only after everything else was successful
	err = c.Options.FS.Rename(
		chapterTempPath,
		chapterPath,
	)
	if err != nil {
		return "", err
	}

	if options.ReadAfter {
		return chapterPath, c.readChapter(ctx, chapterPath, chapter, options.ReadIncognito)
	}

	return chapterPath, nil
}

// DownloadPagesInBatch downloads multiple pages in batch
// by calling DownloadPage for each page in a separate goroutines.
// If any of the pages fails to download it will stop downloading other pages
// and return error immediately
func (c Client) DownloadPagesInBatch(
	ctx context.Context,
	pages []Page,
) ([]PageWithImage, error) {
	c.Options.Log(fmt.Sprintf("Downloading %d pages", len(pages)))

	g, _ := errgroup.WithContext(ctx)

	downloadedPages := make([]PageWithImage, len(pages))

	for i, page := range pages {
		// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		i, page := i, page
		g.Go(func() error {
			c.Options.Log(fmt.Sprintf("Page #%03d: downloading", i+1))
			downloaded, err := c.DownloadPage(ctx, page)
			if err != nil {
				return err
			}

			c.Options.Log(fmt.Sprintf("Page #%03d: done", i+1))

			downloadedPages[i] = downloaded

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return downloadedPages, nil
}

// SavePDF saves pages in FormatPDF
func (c Client) SavePDF(
	pages []PageWithImage,
	out io.Writer,
) error {
	c.Options.Log(fmt.Sprintf("Saving %d pages as PDF", len(pages)))

	// convert to readers
	var images = make([]io.Reader, len(pages))
	for i, page := range pages {
		images[i] = bytes.NewReader(page.GetImage())
	}

	return api.ImportImages(nil, out, images, nil, nil)
}

// SaveCBZ saves pages in FormatCBZ
func (c Client) SaveCBZ(
	pages []PageWithImage,
	out io.Writer,
	comicInfoXml ComicInfoXML,
	options ComicInfoXMLOptions,
) error {
	c.Options.Log(fmt.Sprintf("Saving %d pages as CBZ", len(pages)))

	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	for i, page := range pages {
		c.Options.Log(fmt.Sprintf("Saving page #%d", i))

		writer, err := zipWriter.CreateHeader(&zip.FileHeader{
			Name:   fmt.Sprintf("%04d%s", i+1, page.GetExtension()),
			Method: zip.Store,
		})

		if err != nil {
			return err
		}

		_, err = writer.Write(page.GetImage())
		if err != nil {
			return err
		}
	}

	wrapper := comicInfoXml.wrapper(options)
	wrapper.PageCount = len(pages)
	marshalled, err := wrapper.marshal()
	if err != nil {
		return err
	}

	writer, err := zipWriter.CreateHeader(&zip.FileHeader{
		Name:   filenameComicInfoXML,
		Method: zip.Store,
	})
	if err != nil {
		return err
	}

	_, err = writer.Write(marshalled)
	if err != nil {
		return err
	}

	return nil
}

// SaveImages saves pages in FormatImages
func (c Client) SaveImages(
	pages []PageWithImage,
	dir string,
) error {
	c.Options.Log(fmt.Sprintf("Saving %d pages as images dir", len(pages)))
	err := c.Options.FS.MkdirAll(dir, modeDir)
	if err != nil {
		return err
	}

	for i, page := range pages {
		c.Options.Log(fmt.Sprintf("Saving page #%d", i))

		var file afero.File
		file, err = c.Options.FS.Create(filepath.Join(dir, fmt.Sprintf("%04d%s", i+1, page.GetExtension())))
		if err != nil {
			return err
		}

		_, err = file.Write(page.GetImage())
		if err != nil {
			return err
		}

		_ = file.Close()
	}

	return nil
}

// DownloadPage downloads a page contents (image)
func (c Client) DownloadPage(ctx context.Context, page Page) (PageWithImage, error) {
	if withImage, ok := page.(PageWithImage); ok {
		return withImage, nil
	}

	image, err := c.provider.GetPageImage(ctx, c.Options.Log, page)
	if err != nil {
		return nil, err
	}

	return &pageWithImage{
		Page:  page,
		image: image,
	}, nil
}

type Filenames struct {
	Manga, Volume, Chapter string
}

// ComputeFilenames will apply name templates for chapter and manga
// and return resulting strings.
func (c Client) ComputeFilenames(
	chapter Chapter,
	format Format,
) (filenames Filenames) {
	volume := chapter.Volume()
	manga := volume.Manga()

	filenames.Manga = c.Options.MangaNameTemplate(
		c.String(),
		manga,
	)

	filenames.Volume = c.Options.VolumeNameTemplate(
		c.String(),
		volume,
	)

	filenames.Chapter = c.Options.ChapterNameTemplate(
		c.String(),
		chapter,
	)

	filenames.Chapter += format.Extension()

	return filenames
}
