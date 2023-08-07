package libmangal

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/afero"
	"io"
	"math"
	"net/http"
	"path/filepath"
	"time"
)

type pathExistsFunc func(string) (bool, error)

// removeChapter will remove chapter at given path.
// Doesn't matter if it's a directory or a file.
func (c *Client) removeChapter(chapterPath string) error {
	c.logger.Log("Removing " + chapterPath)

	isDir, err := afero.IsDir(c.options.FS, chapterPath)
	if err != nil {
		return err
	}

	if isDir {
		return c.options.FS.RemoveAll(chapterPath)
	}

	return c.options.FS.Remove(chapterPath)
}

// downloadMangaImage will download image related to manga.
// For example this can be either banner image or cover image.
// Manga is required to set Referer header.
func (c *Client) downloadMangaImage(ctx context.Context, manga Manga, URL string, out io.Writer) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return err
	}

	request.Header.Set("Referer", manga.Info().URL)
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

	_, err = io.Copy(out, response.Body)
	return err
}

// downloadCover will download cover if it doesn't exist
func (c *Client) downloadCover(ctx context.Context, manga Manga, out io.Writer) error {
	c.logger.Log("Downloading cover")

	coverURL, ok, err := c.getCoverURL(ctx, manga)
	if err != nil {
		return err
	}
	c.logger.Log(coverURL)

	if !ok {
		return errors.New("cover url not found")
	}

	return c.downloadMangaImage(ctx, manga, coverURL, out)
}

// downloadBanner will download banner if it doesn't exist
func (c *Client) downloadBanner(ctx context.Context, manga Manga, out io.Writer) error {
	c.logger.Log("Downloading banner")

	bannerURL, ok, err := c.getBannerURL(ctx, manga)
	if err != nil {
		return err
	}
	c.logger.Log(bannerURL)

	if !ok {
		return errors.New("cover url not found")
	}

	return c.downloadMangaImage(ctx, manga, bannerURL, out)
}

func (c *Client) getCoverURL(ctx context.Context, manga Manga) (string, bool, error) {
	coverURL := manga.Info().Cover
	if coverURL != "" {
		return coverURL, true, nil
	}

	mangaWithAnilist, ok, err := c.Anilist().MakeMangaWithAnilist(ctx, manga)
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

func (c *Client) getBannerURL(ctx context.Context, manga Manga) (string, bool, error) {
	bannerURL := manga.Info().Banner
	if bannerURL != "" {
		return bannerURL, true, nil
	}

	mangaWithAnilist, ok, err := c.Anilist().MakeMangaWithAnilist(ctx, manga)
	if err != nil {
		return "", false, err
	}

	if !ok {
		return "", false, nil
	}

	bannerURL = mangaWithAnilist.Anilist.BannerImage
	if bannerURL != "" {
		return bannerURL, true, nil
	}

	return "", false, nil
}

// getSeriesJSON gets SeriesJSON from the chapter.
// It tries to check if chapter manga implements MangaWithSeriesJSON
// in case of failure it will fetch manga from anilist.
func (c *Client) getSeriesJSON(ctx context.Context, manga Manga) (SeriesJSON, error) {
	withSeriesJSON, ok := manga.(MangaWithSeriesJSON)
	if ok {
		seriesJSON, err := withSeriesJSON.SeriesJSON()
		if err != nil {
			return SeriesJSON{}, err
		}

		if ok {
			return seriesJSON, nil
		}
	}

	withAnilist, ok, err := c.Anilist().MakeMangaWithAnilist(ctx, manga)
	if err != nil {
		return SeriesJSON{}, err
	}

	if !ok {
		return SeriesJSON{}, errors.New("can't gen series.json from manga")
	}

	return withAnilist.SeriesJSON(), nil
}

func (c *Client) writeSeriesJSON(ctx context.Context, manga Manga, out io.Writer) error {
	c.logger.Log(fmt.Sprintf("Writing %s", filenameSeriesJSON))

	seriesJSON, err := c.getSeriesJSON(ctx, manga)
	if err != nil {
		return err
	}

	marshalled, err := seriesJSON.wrapper().marshal()
	if err != nil {
		return err
	}

	_, err = out.Write(marshalled)
	return err
}

// downloadChapter is a helper function for DownloadChapter
func (c *Client) downloadChapter(
	ctx context.Context,
	chapter Chapter,
	path string,
	options DownloadOptions,
) error {
	pages, err := c.ChapterPages(ctx, chapter)
	if err != nil {
		return err
	}

	downloadedPages, err := c.DownloadPagesInBatch(ctx, pages)
	if err != nil {
		return err
	}

	for _, page := range downloadedPages {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		image, err := options.ImageTransformer(page.GetImage())
		if err != nil {
			return err
		}

		page.SetImage(image)
	}

	switch options.Format {
	case FormatPDF:
		file, err := c.options.FS.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()

		return c.savePDF(downloadedPages, file)
	case FormatTAR:
		file, err := c.options.FS.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()

		return c.saveTAR(downloadedPages, file)
	case FormatTARGZ:
		file, err := c.options.FS.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()

		return c.saveTARGZ(downloadedPages, file)
	case FormatZIP:
		file, err := c.options.FS.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()

		return c.saveZIP(downloadedPages, file)
	case FormatCBZ:
		comicInfoXML, err := c.getComicInfoXML(ctx, chapter)
		if err != nil && options.Strict {
			return err
		}

		file, err := c.options.FS.Create(path)
		if err != nil {
			return err
		}
		defer file.Close()

		return c.saveCBZ(downloadedPages, file, comicInfoXML, options.ComicInfoXMLOptions)
	case FormatImages:
		if err := c.options.FS.MkdirAll(path, modeDir); err != nil {
			return err
		}

		for i, page := range downloadedPages {
			name := fmt.Sprintf("%04d%s", i+1, page.GetExtension())
			err := afero.WriteFile(
				c.options.FS,
				filepath.Join(path, name),
				page.GetImage(),
				modeFile,
			)
			if err != nil {
				return err
			}
		}

		return nil
	default:
		// format validation was done before
		panic("unreachable")
	}
}

func (c *Client) getComicInfoXML(
	ctx context.Context,
	chapter Chapter,
) (ComicInfoXML, error) {
	withComicInfoXML, ok := chapter.(ChapterWithComicInfoXML)
	if ok {
		comicInfo, err := withComicInfoXML.ComicInfoXML()
		if err != nil {
			return ComicInfoXML{}, err
		}

		return comicInfo, nil
	}

	chapterWithAnilist, ok, err := c.Anilist().MakeChapterWithAnilist(ctx, chapter)
	if err != nil {
		return ComicInfoXML{}, err
	}

	if !ok {
		return ComicInfoXML{}, errors.New("can't get ComicInfo")
	}

	return chapterWithAnilist.ComicInfoXML(), nil
}

func (c *Client) ReadChapter(ctx context.Context, path string, chapter Chapter, options ReadOptions) error {
	c.logger.Log("Opening chapter with the default app")

	err := open.Run(path)
	if err != nil {
		return err
	}

	if options.SaveAnilist && c.Anilist().IsAuthorized() {
		return c.markChapterAsRead(ctx, chapter)
	}

	// TODO: save to local history

	return nil
}

func (c *Client) markChapterAsRead(ctx context.Context, chapter Chapter) error {
	chapterMangaInfo := chapter.Volume().Manga().Info()

	var titleToSearch string

	if title := chapterMangaInfo.AnilistSearch; title != "" {
		titleToSearch = title
	} else if title := chapterMangaInfo.Title; title != "" {
		titleToSearch = title
	} else {
		return fmt.Errorf("can't find title for chapter %q", chapter)
	}

	manga, ok, err := c.Anilist().FindClosestManga(ctx, titleToSearch)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("manga for chapter %q was not found on anilist", chapter)
	}

	progress := int(math.Trunc(float64(chapter.Info().Number)))
	return c.Anilist().SetMangaProgress(ctx, manga.ID, progress)
}

// savePDF saves pages in FormatPDF
func (c *Client) savePDF(
	pages []PageWithImage,
	out io.Writer,
) error {
	c.logger.Log(fmt.Sprintf("Saving %d pages as PDF", len(pages)))

	// convert to readers
	var images = make([]io.Reader, len(pages))
	for i, page := range pages {
		images[i] = bytes.NewReader(page.GetImage())
	}

	return api.ImportImages(nil, out, images, nil, nil)
}

// saveCBZ saves pages in FormatCBZ
func (c *Client) saveCBZ(
	pages []PageWithImage,
	out io.Writer,
	comicInfoXml ComicInfoXML,
	options ComicInfoXMLOptions,
) error {
	c.logger.Log(fmt.Sprintf("Saving %d pages as CBZ", len(pages)))

	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	for i, page := range pages {
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

func (c *Client) saveTAR(
	pages []PageWithImage,
	out io.Writer,
) error {
	tarWriter := tar.NewWriter(out)
	defer tarWriter.Close()

	for i, page := range pages {
		image := page.GetImage()
		err := tarWriter.WriteHeader(&tar.Header{
			Name:    fmt.Sprintf("%04d%s", i+1, page.GetExtension()),
			Size:    int64(len(image)),
			Mode:    0644,
			ModTime: time.Now(),
		})
		if err != nil {
			return err
		}

		_, err = tarWriter.Write(image)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Client) saveTARGZ(
	pages []PageWithImage,
	out io.Writer,
) error {
	gzipWriter := gzip.NewWriter(out)
	defer gzipWriter.Close()

	return c.saveTAR(pages, gzipWriter)
}

func (c *Client) saveZIP(
	pages []PageWithImage,
	out io.Writer,
) error {
	zipWriter := zip.NewWriter(out)
	defer zipWriter.Close()

	for i, page := range pages {
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

	return nil
}

func (c *Client) downloadChapterWithMetadata(
	ctx context.Context,
	chapter Chapter,
	options DownloadOptions,
	existsFunc pathExistsFunc,
) (string, error) {
	directory := options.Directory

	var (
		seriesJSONDir = directory
		coverDir      = directory
		bannerDir     = directory
	)

	if options.CreateMangaDir {
		directory = filepath.Join(directory, c.ComputeMangaFilename(chapter.Volume().Manga()))
		seriesJSONDir = directory
		coverDir = directory
		bannerDir = directory
	}

	if options.CreateVolumeDir {
		directory = filepath.Join(directory, c.ComputeVolumeFilename(chapter.Volume()))
	}

	err := c.options.FS.MkdirAll(directory, modeDir)
	if err != nil {
		return "", err
	}

	chapterPath := filepath.Join(directory, c.ComputeChapterFilename(chapter, options.Format))

	chapterExists, err := existsFunc(chapterPath)
	if err != nil {
		return "", err
	}

	if !chapterExists || !options.SkipIfExists {
		err = c.downloadChapter(ctx, chapter, chapterPath, options)
		if err != nil {
			return "", err
		}
	}

	if options.WriteSeriesJson {
		path := filepath.Join(seriesJSONDir, filenameSeriesJSON)
		exists, err := existsFunc(path)
		if err != nil {
			return "", err
		}

		if !exists {
			file, err := c.options.FS.Create(path)
			if err != nil {
				return "", err
			}
			defer file.Close()

			err = c.writeSeriesJSON(ctx, chapter.Volume().Manga(), file)
			if err != nil && options.Strict {
				return "", MetadataError{err}
			}
		}
	}

	if options.DownloadMangaCover {
		path := filepath.Join(coverDir, filenameCoverJPG)
		exists, err := existsFunc(path)
		if err != nil {
			return "", err
		}

		if !exists {
			file, err := c.options.FS.Create(path)
			if err != nil {
				return "", err
			}
			defer file.Close()

			err = c.downloadCover(ctx, chapter.Volume().Manga(), file)
			if err != nil && options.Strict {
				return "", MetadataError{err}
			}
		}
	}

	if options.DownloadMangaBanner {
		path := filepath.Join(bannerDir, filenameBannerJPG)
		exists, err := existsFunc(path)
		if err != nil {
			return "", err
		}

		file, err := c.options.FS.Create(path)
		if err != nil {
			return "", err
		}
		defer file.Close()

		if !exists {
			err = c.downloadBanner(ctx, chapter.Volume().Manga(), file)
			if err != nil && options.Strict {
				return "", MetadataError{err}
			}
		}
	}

	return chapterPath, nil
}
