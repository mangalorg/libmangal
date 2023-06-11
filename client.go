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
func NewClient[M Manga, V Volume, C Chapter, P Page](
	ctx context.Context,
	loader ProviderLoader[M, V, C, P],
	options ClientOptions,
) (Client[M, V, C, P], error) {
	if err := loader.Info().Validate(); err != nil {
		return Client[M, V, C, P]{}, err
	}

	provider, err := loader.Load(ctx)
	if err != nil {
		return Client[M, V, C, P]{}, err
	}

	return Client[M, V, C, P]{
		provider: provider,
		options:  options,
	}, nil
}

// Client is the wrapper around Provider with the extended functionality.
// It's the core of the libmangal
type Client[M Manga, V Volume, C Chapter, P Page] struct {
	rawScript []byte
	provider  Provider[M, V, C, P]
	options   ClientOptions
}

// SearchMangas searches for mangas with the given query
func (c Client[M, V, C, P]) SearchMangas(ctx context.Context, query string) ([]M, error) {
	return c.provider.SearchMangas(ctx, c.options.Log, query)
}

// MangaVolumes gets chapters of the given manga
func (c Client[M, V, C, P]) MangaVolumes(ctx context.Context, manga M) ([]V, error) {
	return c.provider.MangaVolumes(ctx, c.options.Log, manga)
}

// VolumeChapters gets chapters of the given manga
func (c Client[M, V, C, P]) VolumeChapters(ctx context.Context, volume V) ([]C, error) {
	return c.provider.VolumeChapters(ctx, c.options.Log, volume)
}

// ChapterPages gets pages of the given chapter
func (c Client[M, V, C, P]) ChapterPages(ctx context.Context, chapter C) ([]P, error) {
	return c.provider.ChapterPages(ctx, c.options.Log, chapter)
}

func (c Client[M, V, C, P]) String() string {
	return c.provider.Info().Name
}

// Info returns info about provider
func (c Client[M, V, C, P]) Info() ProviderInfo {
	return c.provider.Info()
}

// DownloadChapter downloads and saves chapter to the specified
// directory in the given format.
//
// It will return resulting chapter path joined with DownloadOptions.Directory
func (c Client[M, V, C, P]) DownloadChapter(
	ctx context.Context,
	chapter C,
	options DownloadOptions,
) (string, error) {
	c.options.Log(fmt.Sprintf("Downloading chapter %q as %s", chapter.Info().Title, options.Format.String()))

	filenames := c.ComputeFilenames(chapter, options.Format)

	if options.CreateMangaDir {
		options.Directory = filepath.Join(options.Directory, filenames.Manga)
	}

	if options.CreateVolumeDir {
		options.Directory = filepath.Join(options.Directory, filenames.Volume)
	}

	err := c.options.FS.MkdirAll(options.Directory, modeDir)
	if err != nil {
		return "", err
	}

	chapterPath := filepath.Join(options.Directory, filenames.Chapter)

	chapterExists, err := afero.Exists(c.options.FS, chapterPath)
	if err != nil {
		return "", err
	}

	if chapterExists && options.SkipIfExists {
		c.options.Log(fmt.Sprintf("Chapter %q already exists, skipping", chapter.Info().Title))

		if options.ReadAfter {
			return chapterPath, c.readChapter(chapterPath, options.ReadIncognito)
		}

		return chapterPath, nil
	}

	// create a temp dir where chapter will be downloaded.
	// after successful download chapter will be moved to the original location
	tempDir, err := afero.TempDir(c.options.FS, "", "")
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
		err := c.writeSeriesJson(ctx, chapter, options.Directory)
		if err != nil && options.Strict {
			return "", err
		}
	}

	if options.DownloadMangaCover {
		err := c.downloadCover(ctx, chapter, options.Directory)
		if err != nil && options.Strict {
			return "", err
		}
	}

	if options.DownloadMangaBanner {
		err := c.downloadBanner(ctx, chapter, options.Directory)
		if err != nil && options.Strict {
			return "", err
		}
	}

	// move to the original location
	// only after everything else was successful
	err = c.options.FS.Rename(
		chapterTempPath,
		chapterPath,
	)
	if err != nil {
		return "", err
	}

	if options.ReadAfter {
		return chapterPath, c.readChapter(chapterPath, options.ReadIncognito)
	}

	return chapterPath, nil
}

// DownloadPagesInBatch downloads multiple pages in batch
// by calling DownloadPage for each page in a separate goroutines.
// If any of the pages fails to download it will stop downloading other pages
// and return error immediately
func (c Client[M, V, C, P]) DownloadPagesInBatch(
	ctx context.Context,
	pages []P,
) ([]PageWithImage[P], error) {
	c.options.Log(fmt.Sprintf("Downloading %d pages", len(pages)))

	g, _ := errgroup.WithContext(ctx)

	downloadedPages := make([]PageWithImage[P], len(pages))

	for i, page := range pages {
		// https://github.com/golang/go/wiki/CommonMistakes#using-goroutines-on-loop-iterator-variables
		i, page := i, page
		g.Go(func() error {
			c.options.Log(fmt.Sprintf("Page #%03d: downloading", i+1))
			downloaded, err := c.DownloadPage(ctx, page)
			if err != nil {
				return err
			}

			c.options.Log(fmt.Sprintf("Page #%03d: done", i+1))

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
func (c Client[M, V, C, P]) SavePDF(
	pages []PageWithImage[P],
	path string,
) error {
	c.options.Log(fmt.Sprintf("Saving %d pages as PDF", len(pages)))

	var file afero.File
	file, err := c.options.FS.Create(path)
	if err != nil {
		return err
	}

	defer file.Close()

	// convert to readers
	var images = make([]io.Reader, len(pages))
	for i, page := range pages {
		images[i] = bytes.NewReader(page.Image)
	}

	return api.ImportImages(nil, file, images, nil, nil)
}

// SaveCBZ saves pages in FormatCBZ
func (c Client[M, V, C, P]) SaveCBZ(
	pages []PageWithImage[P],
	path string,
	comicInfoXml ComicInfoXml,
	options ComicInfoXmlOptions,
) error {
	c.options.Log(fmt.Sprintf("Saving %d pages as CBZ", len(pages)))

	var file afero.File
	file, err := c.options.FS.Create(path)
	if err != nil {
		return err
	}

	defer file.Close()

	zipWriter := zip.NewWriter(file)
	defer zipWriter.Close()

	for i, page := range pages {
		c.options.Log(fmt.Sprintf("Saving page #%d", i))

		if page.Image == nil {
			// this should not occur, but just for the safety
			return fmt.Errorf("image %d is nil", i)
		}

		var writer io.Writer
		writer, err = zipWriter.CreateHeader(&zip.FileHeader{
			Name:   fmt.Sprintf("%04d%s", i+1, page.Page.GetExtension()),
			Method: zip.Store,
		})

		if err != nil {
			return err
		}

		_, err = writer.Write(page.Image)
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
		Name:   comicInfoXmlFilename,
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
func (c Client[M, V, C, P]) SaveImages(
	pages []PageWithImage[P],
	path string,
) error {
	c.options.Log(fmt.Sprintf("Saving %d pages as images dir", len(pages)))
	err := c.options.FS.MkdirAll(path, modeDir)
	if err != nil {
		return err
	}

	for i, page := range pages {
		c.options.Log(fmt.Sprintf("Saving page #%d", i))

		if page.Image == nil {
			// this should not occur, but just for the safety
			return fmt.Errorf("reader %d is nil", i)
		}

		var file afero.File
		file, err = c.options.FS.Create(filepath.Join(path, fmt.Sprintf("%04d%s", i+1, page.Page.GetExtension())))
		if err != nil {
			return err
		}

		_, err = file.Write(page.Image)
		if err != nil {
			return err
		}

		_ = file.Close()
	}

	return nil
}

// DownloadPage downloads a page contents (image)
func (c Client[M, V, C, P]) DownloadPage(ctx context.Context, page P) (PageWithImage[P], error) {
	image, err := c.provider.GetPageImage(ctx, c.options.Log, page)
	if err != nil {
		return PageWithImage[P]{}, err
	}

	return PageWithImage[P]{
		Page:  page,
		Image: image,
	}, nil
}

type Filenames struct {
	Manga, Volume, Chapter string
}

// ComputeFilenames will apply name templates for chapter and manga
// and return resulting strings.
func (c Client[M, V, C, P]) ComputeFilenames(
	chapter C,
	format Format,
) (filenames Filenames) {
	volume := chapter.Volume()
	manga := volume.Manga()

	filenames.Manga = c.options.MangaNameTemplate(
		c.String(),
		manga,
	)

	filenames.Volume = c.options.VolumeNameTemplate(
		c.String(),
		volume,
	)

	filenames.Chapter = c.options.ChapterNameTemplate(
		c.String(),
		chapter,
	)

	extension, ok := FormatExtensions[format]
	if ok {
		filenames.Chapter += extension
	}

	return filenames
}
