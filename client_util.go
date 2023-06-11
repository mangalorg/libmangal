package libmangal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/afero"
	"net/http"
	"path/filepath"
)

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

// downloadCover will download cover if it doesn't exist
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

// downloadBanner will download banner if it doesn't exist
func (c Client[M, V, C, P]) downloadBanner(ctx context.Context, chapter C, dir string) error {
	c.options.Log("Downloading banner")

	bannerPath := filepath.Join(dir, bannerJpgFilename)

	exists, err := afero.Exists(c.options.FS, bannerPath)
	if err != nil {
		return err
	}

	if exists {
		c.options.Log("Banner is already downloaded, skipping")
		return nil
	}

	coverURL, ok, err := c.getBannerURL(ctx, chapter)
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

	c.options.Log("Banner downloaded")
	return afero.WriteFile(c.options.FS, bannerPath, cover, modeFile)
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

func (c Client[M, V, C, P]) getBannerURL(ctx context.Context, chapter C) (string, bool, error) {
	manga := chapter.Volume().Manga()

	bannerURL := manga.Info().Banner
	if bannerURL != "" {
		return bannerURL, true, nil
	}

	mangaWithAnilist, ok, err := c.options.Anilist.MakeMangaWithAnilist(ctx, manga)
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

func (c Client[M, V, C, P]) readChapter(path string, incognito bool) error {
	c.options.Log("Opening chapter with the default app")
	// TODO: history? anilist sync?

	err := open.Run(path)
	if err != nil {
		return err
	}

	return nil
}
