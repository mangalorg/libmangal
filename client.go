package libmangal

import (
	"archive/zip"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/skratchdot/open-golang/open"
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
	options *ClientOptions,
) (*Client, error) {
	if err := loader.Info().Validate(); err != nil {
		return nil, err
	}

	provider, err := loader.Load(ctx)
	if err != nil {
		return nil, err
	}

	return &Client{
		provider: provider,
		options:  options,
	}, nil
}

// Client is the wrapper around Provider with the extended functionality.
// It's the core of the libmangal
type Client struct {
	rawScript []byte
	provider  Provider
	options   *ClientOptions
}

// SearchMangas searches for mangas with the given query
func (c *Client) SearchMangas(ctx context.Context, query string) ([]Manga, error) {
	return c.provider.SearchMangas(ctx, c.options.Log, query)
}

// MangaChapters gets chapters of the given manga
func (c *Client) MangaChapters(ctx context.Context, manga Manga) ([]Chapter, error) {
	return c.provider.MangaChapters(ctx, c.options.Log, manga)
}

// ChapterPages gets pages of the given chapter
func (c *Client) ChapterPages(ctx context.Context, chapter Chapter) ([]Page, error) {
	return c.provider.ChapterPages(ctx, c.options.Log, chapter)
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
func (c *Client) DownloadChapter(
	ctx context.Context,
	chapter Chapter,
	dir string,
	options *DownloadOptions,
) (string, error) {
	if !options.Format.IsAFormat() {
		return "", fmt.Errorf("unsupported format")
	}

	c.options.Log(fmt.Sprintf("Downloading chapter %q as %s", chapter.GetTitle(), options.Format.String()))

	mangaFilename, chapterFilename := c.ComputeFilenames(chapter, options.Format)

	var (
		mangaPath   = filepath.Join(dir, mangaFilename)
		chapterPath = filepath.Join(mangaPath, chapterFilename)
	)

	exists, err := afero.Exists(c.options.FS, chapterPath)
	if err != nil {
		return chapterFilename, nil
	}

	if exists && options.SkipIfExists {
		c.options.Log(fmt.Sprintf("Chapter %q already exists, skipping", chapter.GetTitle()))
		return chapterFilename, nil
	}

	// create a temp dir where chapter will be downloaded.
	// after successful download chapter will be moved to the original location
	tempDir, err := afero.TempDir(c.options.FS, "", "")
	if err != nil {
		return "", err
	}

	var (
		mangaTempPath   = filepath.Join(tempDir, mangaFilename)
		chapterTempPath = filepath.Join(mangaTempPath, chapterFilename)
	)

	err = c.options.FS.MkdirAll(mangaTempPath, 0755)
	if err != nil {
		return "", err
	}

	err = c.downloadChapter(ctx, chapter, chapterTempPath, options)
	if err != nil {
		return "", err
	}

	err = c.options.FS.MkdirAll(mangaPath, 0755)
	if err != nil {
		return "", err
	}

	if exists {
		c.options.Log(fmt.Sprintf("Chapter %q already exists, removing", chapter.GetTitle()))
		if options.Format == FormatImages {
			err = c.options.FS.RemoveAll(chapterPath)
		} else {
			err = c.options.FS.Remove(chapterPath)
		}

		if err != nil {
			return "", err
		}
	}

	if err != nil {
		return "", err
	}

	if options.WriteSeriesJson {
		manga, err := c.options.Anilist.MakeMangaWithAnilist(ctx, chapter.GetManga())
		if err != nil {
			return "", err
		}

		seriesJson := manga.SeriesJson()
		seriesJsonPath := filepath.Join(mangaPath, seriesJsonFilename)

		marshalled, err := json.Marshal(seriesJson)
		if err != nil {
			return "", err
		}

		err = afero.WriteFile(c.options.FS, seriesJsonPath, marshalled, 0644)
		if err != nil {
			return "", err
		}
	}

	if options.DownloadMangaCover {
		err = c.downloadCoverIfNotExists(
			ctx,
			chapter.GetManga(),
			mangaPath,
		)

		if err != nil {
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

// downloadCoverIfNotExists will download manga chapter
// and save it as a cover.jpg in the specified directory.
// It will also try to convert Manga to MangaWithAnilist
// to obtain a better cover image. If it fails to do so,
// then it will use Manga.CoverUrl.
func (c *Client) downloadCoverIfNotExists(
	ctx context.Context,
	manga Manga,
	dir string,
) error {
	return nil

	//const coverFilename = "cover.jpg"
	//
	//exists, err := afero.Exists(c.options.FS, filepath.Join(dir, coverFilename))
	//if err != nil {
	//	return err
	//}
	//
	//if exists {
	//	return nil
	//}
	//
	//var coverUrl string
	//
	//if manga, err := c.MakeMangaWithAnilist(ctx, manga); err != nil {
	//	for _, url := range []string{
	//		manga.Anilist.CoverImage.ExtraLarge,
	//		// no need to check `Large` cover, see `ExtraLarge` description
	//		manga.Anilist.CoverImage.Medium,
	//		manga.GetCoverURL(),
	//	} {
	//		if url != "" {
	//			coverUrl = url
	//			break
	//		}
	//	}
	//} else {
	//	coverUrl = manga.GetCoverURL()
	//}
	//
	//if coverUrl == "" {
	//	return fmt.Errorf("cover url is empty")
	//}
	//
	//c.options.Log("downloading cover")
	//
	//panic("unimplemented")

	//
	//cover := Page{
	//	Url: coverUrl,
	//}
	//
	//downloaded, err := c.DownloadPage(ctx, &cover)
	//if err != nil {
	//	return err
	//}
	//
	//return afero.WriteReader(
	//	c.options.FS,
	//	filepath.Join(dir, coverFilename),
	//	downloaded,
	//)
}

// downloadChapter is a helper function for DownloadChapter
func (c *Client) downloadChapter(
	ctx context.Context,
	chapter Chapter,
	path string,
	options *DownloadOptions,
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
		var comicInfo *ComicInfoXml
		if options.WriteComicInfoXml {
			chapter, err := c.options.Anilist.MakeChapterWithAnilist(ctx, chapter)
			if err != nil {
				return err
			}

			comicInfo = chapter.ComicInfoXml(options.ComicInfoOptions)
		}

		err = c.SaveCBZ(downloadedPages, path, comicInfo)
	case FormatImages:
		err = c.SaveImages(downloadedPages, path)
	}

	if err != nil {
		return err
	}

	return nil
}

// DownloadPagesInBatch downloads multiple pages in batch
// by calling DownloadPage for each page in a separate goroutines.
// If any of the pages fails to download it will stop downloading other pages
// and return error immediately
func (c *Client) DownloadPagesInBatch(
	ctx context.Context,
	pages []Page,
) ([]*PageWithImage, error) {
	c.options.Log(fmt.Sprintf("Downloading %d pages", len(pages)))

	g, _ := errgroup.WithContext(ctx)

	downloadedPages := make([]*PageWithImage, len(pages))

	for i, page := range pages {
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
func (c *Client) SavePDF(
	pages []*PageWithImage,
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
		images[i] = page.Reader
	}

	return api.ImportImages(nil, file, images, nil, nil)
}

// SaveCBZ saves pages in FormatCBZ
func (c *Client) SaveCBZ(
	pages []*PageWithImage,
	path string,
	comicInfoXml *ComicInfoXml,
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

		if page.Reader == nil {
			// this should not occur, but just for the safety
			return fmt.Errorf("reader %d is nil", i)
		}

		var writer io.Writer
		writer, err = zipWriter.CreateHeader(&zip.FileHeader{
			Name:   fmt.Sprintf("%04d%s", i+1, page.GetExtension()),
			Method: zip.Store,
		})

		if err != nil {
			return err
		}

		_, err = io.Copy(writer, page.Reader)
		if err != nil {
			return err
		}
	}

	if comicInfoXml != nil {
		marshalled, err := xml.MarshalIndent(comicInfoXml, "", "  ")
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
	}

	return nil
}

// SaveImages saves pages in FormatImages
func (c *Client) SaveImages(
	pages []*PageWithImage,
	path string,
) error {
	c.options.Log(fmt.Sprintf("Saving %d pages as images dir", len(pages)))
	err := c.options.FS.MkdirAll(path, 0755)
	if err != nil {
		return err
	}

	for i, page := range pages {
		c.options.Log(fmt.Sprintf("Saving page #%d", i))

		if page.Reader == nil {
			// this should not occur, but just for the safety
			return fmt.Errorf("reader %d is nil", i)
		}

		var file afero.File
		file, err = c.options.FS.Create(filepath.Join(path, fmt.Sprintf("%04d%s", i+1, page.GetExtension())))
		if err != nil {
			return err
		}

		_, err = io.Copy(file, page)
		if err != nil {
			return err
		}

		_ = file.Close()
	}

	return nil
}

// DownloadPage downloads a page contents (image)
func (c *Client) DownloadPage(ctx context.Context, page Page) (*PageWithImage, error) {
	reader, err := c.provider.GetImage(ctx, c.options.Log, page)
	if err != nil {
		return nil, err
	}

	return &PageWithImage{
		Page:   page,
		Reader: reader,
	}, nil
}

// ReadChapter downloads chapter to the temp directory and opens it with the
// os default app for resulting mimetype.
// E.g. `xdg-open` for Linux.
//
// Note: works only for afero.OsFs
func (c *Client) ReadChapter(ctx context.Context, chapter Chapter, options *ReadOptions) error {
	if c.options.FS.Name() != "OsFs" {
		return fmt.Errorf("only OsFs is supported for reading")
	}

	c.options.Log(fmt.Sprintf("Reading chapter %q as %s", chapter.GetTitle(), options.Format))

	var chapterPath string
	if options.MangasLibraryPath != "" {
		path, ok, err := c.IsChapterDownloaded(chapter, options.MangasLibraryPath, options.Format)
		if err != nil {
			return err
		}

		if ok {
			c.options.Log(fmt.Sprintf("Chapter %q is already downloaded", chapter.GetTitle()))
			chapterPath = path
		}
	}

	if chapterPath == "" {
		c.options.Log(fmt.Sprintf("Creating temp dir"))
		tempDir, err := afero.TempDir(c.options.FS, "", "libmangal")
		if err != nil {
			return err
		}

		chapterPath, err = c.DownloadChapter(
			ctx,
			chapter,
			tempDir,
			DefaultDownloadOptions(),
		)
		if err != nil {
			return err
		}
	}

	c.options.Log(fmt.Sprintf("Opening chapter %q with default app", chapter.GetTitle()))
	err := open.Run(chapterPath)
	if err != nil {
		return err
	}

	// TODO: history?

	return nil
}

// IsChapterDownloaded checks if chapter is downloaded.
// It will simply check if path dir/manga/chapter exists
func (c *Client) IsChapterDownloaded(
	chapter Chapter,
	dir string,
	format Format,
) (path string, ok bool, err error) {
	mangaFilename, chapterFilename := c.ComputeFilenames(chapter, format)

	path = filepath.Join(dir, mangaFilename, chapterFilename)

	exists, err := afero.Exists(c.options.FS, path)
	if err != nil {
		return "", false, err
	}

	return path, exists, nil
}

// ComputeFilenames will apply name templates for chapter and manga
// and return resulting strings.
func (c *Client) ComputeFilenames(
	chapter Chapter,
	format Format,
) (mangaFilename, chapterFilename string) {
	mangaFilename = c.options.MangaNameTemplate(
		c.String(),
		MangaNameData{
			Title: chapter.GetManga().GetTitle(),
			Id:    chapter.GetManga().GetID(),
		},
	)

	chapterFilename = c.options.ChapterNameTemplate(
		c.String(),
		ChapterNameData{
			Title:      chapter.GetTitle(),
			Number:     chapter.GetNumber(),
			MangaTitle: chapter.GetManga().GetTitle(),
		},
	)

	extension, ok := FormatExtensions[format]
	if ok {
		chapterFilename += extension
	}

	return mangaFilename, chapterFilename
}
