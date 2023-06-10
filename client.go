package libmangal

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"io"
	"net/http"
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
func (c Client[M, V, C, P]) DownloadChapter(
	ctx context.Context,
	chapter C,
	dir string,
	options DownloadOptions,
) (string, error) {
	c.options.Log(fmt.Sprintf("Downloading chapter %q as %s", chapter.Info().Title, options.Format.String()))

	filenames := c.ComputeFilenames(chapter, options.Format)

	if options.CreateMangaDir {
		dir = filepath.Join(dir, filenames.Manga)
	}

	if options.CreateVolumeDir {
		dir = filepath.Join(dir, filenames.Volume)
	}

	err := c.options.FS.MkdirAll(dir, modeDir)
	if err != nil {
		return "", err
	}

	chapterPath := filepath.Join(dir, filenames.Chapter)

	chapterExists, err := afero.Exists(c.options.FS, chapterPath)
	if err != nil {
		return "", err
	}

	if chapterExists && options.SkipIfExists {
		c.options.Log(fmt.Sprintf("Chapter %q already exists, skipping", chapter.Info().Title))

		return filenames.Chapter, nil
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
		err := c.writeSeriesJson(ctx, chapter, dir)
		if err != nil && options.Strict {
			return "", err
		}
	}

	if options.DownloadMangaCover {
		err := c.downloadCover(ctx, chapter, dir)
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

	return chapterPath, nil
}

// removeChapter will remove chapter at given path.
// Doesn't matter if it's a directory or a file.
func (c Client[M, V, C, P]) removeChapter(chapterPath string) error {
	c.options.Log("Removing " + chapterPath)

	isDir, err := afero.IsDir(c.options.FS, chapterPath)
	if err != nil {
		return err
	}

	if isDir {
		return c.options.FS.RemoveAll(chapterPath)
	}

	return c.options.FS.Remove(chapterPath)
}

// downloadCover will download if it doesn't exist
func (c Client[M, V, C, P]) downloadCover(ctx context.Context, chapter C, dir string) error {
	c.options.Log("Downloading cover")

	coverPath := filepath.Join(dir, coverJpgFilename)

	exists, err := afero.Exists(c.options.FS, coverPath)
	if err != nil {
		return err
	}

	if exists {
		c.options.Log("Cover is already downloaded, skipping")
		return nil
	}

	coverURL, ok, err := c.getCoverURL(ctx, chapter)
	if err != nil {
		return err
	}
	c.options.Log(coverURL)

	if !ok {
		return errors.New("cover url not found")
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, coverURL, nil)
	if err != nil {
		return err
	}

	request.Header.Set("Referer", chapter.Volume().Manga().Info().URL)
	request.Header.Set("User-Agent", UserAgent)
	request.Header.Set("Accept", "image/webp,image/apng,image/*,*/*;q=0.8")

	response, err := c.options.HTTPClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected http status: %s", response.Status)
	}

	cover, err := readResponseBody(response.ContentLength, response.Body)
	if err != nil {
		return err
	}

	c.options.Log("Cover downloaded")
	return afero.WriteFile(c.options.FS, coverPath, cover, modeFile)
}

func (c Client[M, V, C, P]) getCoverURL(ctx context.Context, chapter C) (string, bool, error) {
	manga := chapter.Volume().Manga()

	coverURL := manga.Info().Cover
	if coverURL != "" {
		return coverURL, true, nil
	}

	mangaWithAnilist, ok, err := c.options.Anilist.MakeMangaWithAnilist(ctx, manga)
	if err != nil {
		return "", false, err
	}

	if !ok {
		return "", false, nil
	}

	for _, coverURL := range []string{
		mangaWithAnilist.Anilist.CoverImage.ExtraLarge,
		mangaWithAnilist.Anilist.CoverImage.Medium,
	} {
		if coverURL != "" {
			return coverURL, true, nil
		}
	}

	return "", false, nil
}

func (c Client[M, V, C, P]) writeSeriesJson(ctx context.Context, chapter C, dir string) error {
	c.options.Log("Writing series.json")

	seriesJsonPath := filepath.Join(dir, seriesJsonFilename)

	exists, err := afero.Exists(c.options.FS, seriesJsonPath)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	var seriesJson SeriesJson

	seriesJson, ok := chapter.Volume().Manga().SeriesJson()
	if !ok {
		manga, ok, err := c.options.Anilist.MakeMangaWithAnilist(ctx, chapter.Volume().Manga())
		if err != nil {
			return err
		}

		if ok {
			seriesJson = manga.SeriesJson()
		}
	}

	marshalled, err := json.Marshal(seriesJson)
	if err != nil {
		return err
	}

	err = afero.WriteFile(c.options.FS, seriesJsonPath, marshalled, modeFile)
	if err != nil {
		return err
	}

	return nil
}

// downloadChapter is a helper function for DownloadChapter
func (c Client[M, V, C, P]) downloadChapter(
	ctx context.Context,
	chapter C,
	path string,
	options DownloadOptions,
) error {
	pages, err := c.provider.ChapterPages(ctx, c.options.Log, chapter)
	if err != nil {
		return err
	}

	downloadedPages, err := c.DownloadPagesInBatch(ctx, pages)
	if err != nil {
		return err
	}

	switch options.Format {
	case FormatPDF:
		err = c.SavePDF(downloadedPages, path)
	case FormatCBZ:
		comicInfo, err := c.generateComicInfo(ctx, chapter)
		if err != nil && options.Strict {
			return err
		}

		err = c.SaveCBZ(downloadedPages, path, comicInfo, options.ComicInfoOptions)
	case FormatImages:
		err = c.SaveImages(downloadedPages, path)
	}

	if err != nil {
		return err
	}

	return nil
}

func (c Client[M, V, C, P]) generateComicInfo(
	ctx context.Context,
	chapter C,
) (ComicInfoXml, error) {
	if comicInfo, ok := chapter.ComicInfoXml(); ok {
		return comicInfo, nil
	}

	chapterWithAnilist, ok, err := c.options.Anilist.MakeChapterWithAnilist(ctx, chapter)
	if err != nil {
		return ComicInfoXml{}, err
	}

	if !ok {
		return ComicInfoXml{}, nil
	}

	return chapterWithAnilist.ComicInfoXml(), nil
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

// ReadChapter downloads chapter to the temp directory and opens it with the
// os default app for resulting mimetype.
// E.g. `xdg-open` for Linux.
//
// Note: works only for afero.OsFs
func (c Client[M, V, C, P]) ReadChapter(ctx context.Context, chapter C, options ReadOptions) error {
	c.options.Log(fmt.Sprintf("Reading chapter %q as %s", chapter.Info().Title, options.Format))

	if options.MangasLibraryPath != "" {
		filenames := c.ComputeFilenames(chapter, options.Format)

		path := filepath.Join(options.MangasLibraryPath, filenames.Chapter)
		exists, err := afero.Exists(c.options.FS, path)
		if err != nil {
			return err
		}

		if exists {
			c.options.Log(fmt.Sprintf("Chapter %q is already downloaded", chapter.Info().Title))
			return c.readChapter(path)
		}
	}

	c.options.Log(fmt.Sprintf("Creating temp dir"))
	tempDir, err := afero.TempDir(c.options.FS, "", "libmangal")
	if err != nil {
		return err
	}

	path, err := c.DownloadChapter(
		ctx,
		chapter,
		tempDir,
		DownloadOptions{Format: options.Format},
	)

	if err != nil {
		return err
	}

	return c.readChapter(path)
}

func (c Client[M, V, C, P]) readChapter(path string) error {
	c.options.Log("Opening chapter with the default app")
	// TODO: history? anilist sync?

	err := open.Run(path)
	if err != nil {
		return err
	}

	return nil
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
