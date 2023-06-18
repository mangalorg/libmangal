package libmangal

import (
	"context"
	"errors"
	"fmt"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/afero"
	"io"
	"math"
	"net/http"
	"path/filepath"
)

// removeChapter will remove chapter at given path.
// Doesn't matter if it's a directory or a file.
func (c Client) removeChapter(chapterPath string) error {
	c.Options.Log("Removing " + chapterPath)

	isDir, err := afero.IsDir(c.Options.FS, chapterPath)
	if err != nil {
		return err
	}

	if isDir {
		return c.Options.FS.RemoveAll(chapterPath)
	}

	return c.Options.FS.Remove(chapterPath)
}

// downloadCover will download cover if it doesn't exist
func (c Client) downloadCover(ctx context.Context, chapter Chapter, dir string) error {
	c.Options.Log("Downloading cover")

	coverPath := filepath.Join(dir, filenameCoverJPG)

	exists, err := afero.Exists(c.Options.FS, coverPath)
	if err != nil {
		return err
	}

	if exists {
		c.Options.Log("Cover is already downloaded, skipping")
		return nil
	}

	coverURL, ok, err := c.getCoverURL(ctx, chapter)
	if err != nil {
		return err
	}
	c.Options.Log(coverURL)

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

	response, err := c.Options.HTTPClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected http status: %s", response.Status)
	}

	cover, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	c.Options.Log("Cover downloaded")
	return afero.WriteFile(c.Options.FS, coverPath, cover, modeFile)
}

// downloadBanner will download banner if it doesn't exist
func (c Client) downloadBanner(ctx context.Context, chapter Chapter, dir string) error {
	c.Options.Log("Downloading banner")

	bannerPath := filepath.Join(dir, filenameBannerJPG)

	exists, err := afero.Exists(c.Options.FS, bannerPath)
	if err != nil {
		return err
	}

	if exists {
		c.Options.Log("Banner is already downloaded, skipping")
		return nil
	}

	coverURL, ok, err := c.getBannerURL(ctx, chapter)
	if err != nil {
		return err
	}
	c.Options.Log(coverURL)

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

	response, err := c.Options.HTTPClient.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected http status: %s", response.Status)
	}

	cover, err := io.ReadAll(response.Body)
	if err != nil {
		return err
	}

	c.Options.Log("Banner downloaded")
	return afero.WriteFile(c.Options.FS, bannerPath, cover, modeFile)
}

func (c Client) getCoverURL(ctx context.Context, chapter Chapter) (string, bool, error) {
	manga := chapter.Volume().Manga()

	coverURL := manga.Info().Cover
	if coverURL != "" {
		return coverURL, true, nil
	}

	mangaWithAnilist, ok, err := c.Options.Anilist.MakeMangaWithAnilist(ctx, manga)
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

func (c Client) getBannerURL(ctx context.Context, chapter Chapter) (string, bool, error) {
	manga := chapter.Volume().Manga()

	bannerURL := manga.Info().Banner
	if bannerURL != "" {
		return bannerURL, true, nil
	}

	mangaWithAnilist, ok, err := c.Options.Anilist.MakeMangaWithAnilist(ctx, manga)
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
func (c Client) getSeriesJSON(ctx context.Context, manga Manga) (SeriesJSON, error) {
	withSeriesJson, ok := manga.(MangaWithSeriesJSON)
	if ok {
		seriesJson, err := withSeriesJson.SeriesJSON()
		if err != nil {
			return SeriesJSON{}, err
		}

		if ok {
			return seriesJson, nil
		}
	}

	withAnilist, ok, err := c.Options.Anilist.MakeMangaWithAnilist(ctx, manga)
	if err != nil {
		return SeriesJSON{}, err
	}

	if !ok {
		return SeriesJSON{}, errors.New("can't gen series json from manga")
	}

	return withAnilist.SeriesJson(), nil
}

func (c Client) writeSeriesJSON(ctx context.Context, chapter Chapter, dir string) error {
	c.Options.Log("Writing series.json")

	seriesJsonPath := filepath.Join(dir, filenameSeriesJSON)

	exists, err := afero.Exists(c.Options.FS, seriesJsonPath)
	if err != nil {
		return err
	}

	if exists {
		return nil
	}

	seriesJson, err := c.getSeriesJSON(ctx, chapter.Volume().Manga())
	if err != nil {
		return err
	}

	marshalled, err := seriesJson.wrapper().marshal()
	if err != nil {
		return err
	}

	err = afero.WriteFile(c.Options.FS, seriesJsonPath, marshalled, modeFile)
	if err != nil {
		return err
	}

	return nil
}

// downloadChapter is a helper function for DownloadChapter
func (c Client) downloadChapter(
	ctx context.Context,
	chapter Chapter,
	path string,
	options DownloadOptions,
) error {
	pages, err := c.provider.ChapterPages(ctx, c.Options.Log, chapter)
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
		file, err := c.Options.FS.Create(path)
		if err != nil {
			return err
		}

		defer file.Close()

		return c.SavePDF(downloadedPages, file)
	case FormatCBZ:
		comicInfo, err := c.getComicInfoXML(ctx, chapter)
		if err != nil && options.Strict {
			return err
		}

		file, err := c.Options.FS.Create(path)
		if err != nil {
			return err
		}

		defer file.Close()

		return c.SaveCBZ(downloadedPages, file, comicInfo, options.ComicInfoOptions)
	case FormatImages:
		return c.SaveImages(downloadedPages, path)
	default:
		// format validation was done before
		panic("unreachable")
	}
}

func (c Client) getComicInfoXML(
	ctx context.Context,
	chapter Chapter,
) (ComicInfoXML, error) {
	withComicInfo, ok := chapter.(ChapterWithComicInfoXML)
	if ok {
		comicInfo, err := withComicInfo.ComicInfoXML()
		if err != nil {
			return ComicInfoXML{}, err
		}

		return comicInfo, nil
	}

	chapterWithAnilist, ok, err := c.Options.Anilist.MakeChapterWithAnilist(ctx, chapter)
	if err != nil {
		return ComicInfoXML{}, err
	}

	if !ok {
		return ComicInfoXML{}, errors.New("can't get ComicInfo")
	}

	return chapterWithAnilist.ComicInfoXml(), nil
}

func (c Client) readChapter(ctx context.Context, path string, chapter Chapter, incognito bool) error {
	c.Options.Log("Opening chapter with the default app")

	err := open.Run(path)
	if err != nil {
		return err
	}

	if c.Options.Anilist.IsAuthorized() && !incognito {
		return c.markChapterAsRead(ctx, chapter)
	}

	return nil
}

func (c Client) markChapterAsRead(ctx context.Context, chapter Chapter) error {
	chapterMangaInfo := chapter.Volume().Manga().Info()
	titleToSearch := chapterMangaInfo.AnilistSearch
	if titleToSearch == "" {
		titleToSearch = chapterMangaInfo.Title
	}

	manga, ok, err := c.Options.Anilist.FindClosestManga(ctx, titleToSearch)
	if err != nil {
		return err
	}

	if !ok {
		return fmt.Errorf("manga for chapter %q was not found on anilist", chapter)
	}

	progress := int(math.Trunc(float64(chapter.Info().Number)))
	return c.Options.Anilist.SetProgress(ctx, manga.ID, progress)
}
