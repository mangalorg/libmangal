package libmangal

import (
	"fmt"
	"net/http"

	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
	"github.com/spf13/afero"
)

type ReadOptions struct {
	// SaveHistory will save chapter to local history if ReadAfter is enabled.
	SaveHistory bool

	// ReadIncognito will save Anilist reading history if ReadAfter is enabled and logged in to the Anilist.
	SaveAnilist bool
}

func DefaultReadOptions() ReadOptions {
	return ReadOptions{
		SaveHistory: false,
		SaveAnilist: false,
	}
}

// DownloadOptions configures Chapter downloading
type DownloadOptions struct {
	// Format in which a chapter must be downloaded
	Format Format

	// Directory is the directory where manga will be downloaded to
	Directory string

	// CreateMangaDir will create manga directory
	CreateMangaDir bool

	// CreateVolumeDir will create volume directory.
	//
	// If CreateMangaDir is also true, volume directory
	// will be created under it.
	CreateVolumeDir bool

	// Strict means that that if during metadata creation
	// error occurs downloader will return it immediately and chapter
	// won't be downloaded
	Strict bool

	// SkipIfExists will skip downloading chapter if its already downloaded (exists at path)
	//
	// However, metadata will still be created if needed.
	SkipIfExists bool

	// DownloadMangaCover or not. Will not download cover again if its already downloaded.
	DownloadMangaCover bool

	// DownloadMangaBanner or not. Will not download banner again if its already downloaded.
	DownloadMangaBanner bool

	// WriteSeriesJson write metadata series.json file in the manga directory
	WriteSeriesJson bool

	// WriteComicInfoXml write metadata ComicInfo.xml file to the .cbz archive when
	// downloading with FormatCBZ
	WriteComicInfoXml bool

	// ReadAfter will open the chapter for reading after it was downloaded.
	// It will use os default app for resulting mimetype.
	//
	// E.g. `xdg-open` for Linux.
	//
	// It will also sync read chapter with your Anilist profile
	// if it's configured. See also ReadIncognito
	//
	// Note, that underlying filesystem must be mapped with OsFs
	// in order for os to open it.
	ReadAfter bool

	ReadOptions ReadOptions

	// ComicInfoXMLOptions options to use for ComicInfo.xml when WriteComicInfoXml is true
	ComicInfoXMLOptions ComicInfoXMLOptions

	// ImageTransformer is applied for each image for the chapter.
	//
	// E.g. grayscale effect
	ImageTransformer func([]byte) ([]byte, error)
}

// DefaultDownloadOptions constructs default DownloadOptions
func DefaultDownloadOptions() DownloadOptions {
	return DownloadOptions{
		Format:              FormatPDF,
		Directory:           ".",
		CreateMangaDir:      true,
		CreateVolumeDir:     false,
		Strict:              true,
		SkipIfExists:        true,
		DownloadMangaCover:  false,
		DownloadMangaBanner: false,
		WriteSeriesJson:     false,
		WriteComicInfoXml:   false,
		ReadAfter:           false,
		ImageTransformer: func(img []byte) ([]byte, error) {
			return img, nil
		},
		ReadOptions:         DefaultReadOptions(),
		ComicInfoXMLOptions: DefaultComicInfoOptions(),
	}
}

// AnilistOptions is options for Anilist client
type AnilistOptions struct {
	// HTTPClient is a http client used for Anilist API
	HTTPClient *http.Client

	// QueryToIDsStore maps query to ids.
	// single query to multiple ids.
	//
	// ["berserk" => [7, 42, 69], "death note" => [887, 3, 134]]
	QueryToIDsStore gokv.Store

	// TitleToIDStore maps title to id.
	// single title to single id.
	//
	// ["berserk" => 7, "death note" => 3]
	TitleToIDStore gokv.Store

	// IDToMangaStore maps id to manga.
	// single id to single manga.
	//
	// [7 => "{title: ..., image: ..., ...}"]
	IDToMangaStore gokv.Store

	AccessTokenStore gokv.Store

	// LogWriter used for logs progress
	Logger *Logger
}

// DefaultAnilistOptions constructs default AnilistOptions
func DefaultAnilistOptions() AnilistOptions {
	return AnilistOptions{
		Logger: NewLogger(),

		HTTPClient: &http.Client{},

		QueryToIDsStore:  syncmap.NewStore(syncmap.DefaultOptions),
		TitleToIDStore:   syncmap.NewStore(syncmap.DefaultOptions),
		IDToMangaStore:   syncmap.NewStore(syncmap.DefaultOptions),
		AccessTokenStore: syncmap.NewStore(syncmap.DefaultOptions),
	}
}

// ClientOptions is options that client would use during its runtime.
// See DefaultClientOptions
type ClientOptions struct {
	// HTTPClient is http client that client would use for requests
	HTTPClient *http.Client

	// FS is a file system abstraction
	// that the client will use.
	FS afero.Fs

	// ChapterNameTemplate defines how mangas filenames will look when downloaded.
	MangaNameTemplate func(
		provider string,
		manga Manga,
	) string

	// ChapterNameTemplate defines how volumes filenames will look when downloaded.
	// E.g. Vol. 1
	VolumeNameTemplate func(
		provider string,
		volume Volume,
	) string

	// ChapterNameTemplate defines how chapters filenames will look when downloaded.
	// E.g. "[001] chapter 1" or "Chainsaw Man - Ch. 1"
	ChapterNameTemplate func(
		provider string,
		chapter Chapter,
	) string

	// Anilist is the Anilist client to use
	Anilist *Anilist
}

// DefaultClientOptions constructs default ClientOptions
func DefaultClientOptions() ClientOptions {
	anilist := NewAnilist(DefaultAnilistOptions())
	return ClientOptions{
		HTTPClient: &http.Client{},
		FS:         afero.NewOsFs(),
		ChapterNameTemplate: func(_ string, chapter Chapter) string {
			info := chapter.Info()
			number := fmt.Sprintf("%06.1f", info.Number)
			return sanitizePath(fmt.Sprintf("[%s] %s", number, info.Title))
		},
		MangaNameTemplate: func(_ string, manga Manga) string {
			return sanitizePath(manga.Info().Title)
		},
		VolumeNameTemplate: func(_ string, volume Volume) string {
			return sanitizePath(fmt.Sprintf("Vol. %d", volume.Info().Number))
		},
		Anilist: &anilist,
	}
}

// ComicInfoXMLOptions tweaks ComicInfoXML generation
type ComicInfoXMLOptions struct {
	// AddDate whether to add series release date or not
	AddDate bool

	// AlternativeDate use other date
	AlternativeDate *Date
}

// DefaultComicInfoOptions constructs default ComicInfoXMLOptions
func DefaultComicInfoOptions() ComicInfoXMLOptions {
	return ComicInfoXMLOptions{
		AddDate: true,
	}
}
