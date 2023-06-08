package libmangal

import (
	"fmt"
	levenshtein "github.com/ka-weihe/fast-levenshtein"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
	"github.com/samber/lo"
	"github.com/spf13/afero"
	"net/http"
	"strconv"
)

// DownloadOptions configures Chapter downloading
type DownloadOptions struct {
	// Format in which a chapter must be downloaded
	Format Format

	CreateMangaDir  bool
	CreateVolumeDir bool

	// SkipIfExists will skip downloading chapter if its already downloaded (exists)
	SkipIfExists bool

	// DownloadMangaCover or not. Will not download cover again if its already downloaded.
	DownloadMangaCover bool

	// WriteSeriesJson write metadata series.json file in the manga directory
	WriteSeriesJson bool

	// WriteComicInfoXml write metadata ComicInfo.xml file to the .cbz archive when
	// downloading with FormatCBZ
	WriteComicInfoXml bool

	// ComicInfoOptions options to use for ComicInfo.xml when WriteComicInfoXml is true
	ComicInfoOptions *ComicInfoOptions
}

// DefaultDownloadOptions constructs default DownloadOptions
func DefaultDownloadOptions() *DownloadOptions {
	return &DownloadOptions{
		Format:             FormatPDF,
		CreateMangaDir:     true,
		CreateVolumeDir:    false,
		SkipIfExists:       true,
		DownloadMangaCover: false,
		WriteSeriesJson:    false,
		WriteComicInfoXml:  false,
		ComicInfoOptions:   DefaultComicInfoOptions(),
	}
}

// ReadOptions specifies reading options passed to the Client.ReadChapter
type ReadOptions struct {
	// Format used for reading
	Format Format

	// MangasLibraryPath is the path to the directory where mangas are stored.
	// Will be used to see if the given chapter is already downloaded,
	// so it will be opened instead
	MangasLibraryPath string
}

// DefaultReadOptions constructs default ReadOptions
func DefaultReadOptions() *ReadOptions {
	return &ReadOptions{
		Format:            FormatPDF,
		MangasLibraryPath: "",
	}
}

// AnilistOptions is options for Anilist client
type AnilistOptions struct {
	// HTTPClient is a http client used for Anilist API
	HTTPClient *http.Client

	// QueryToIdsStore maps query to ids.
	// single query to multiple ids.
	// ["berserk" => [7, 42, 69], "death note" => [887, 3, 134]]
	QueryToIdsStore gokv.Store

	// TitleToIdStore maps title to id.
	// single title to single id.
	// ["berserk" => 7, "death note" => 3]
	TitleToIdStore gokv.Store

	// IdToMangaStore maps id to manga.
	// single id to single manga.
	// [7 => "{title: ..., image: ..., ...}"]
	IdToMangaStore gokv.Store

	// GetClosestManga is the function that will be used
	// to get the most similar manga from Anilist
	// based on the passed title.
	// If there's no such manga ok will be false
	GetClosestManga func(
		title string,
		anilistMangas []*AnilistManga,
	) (closest *AnilistManga, ok bool)

	// Log logs progress
	Log LogFunc
}

// DefaultAnilistOptions constructs default AnilistOptions
func DefaultAnilistOptions() *AnilistOptions {
	return &AnilistOptions{
		Log: func(string) {},

		HTTPClient: &http.Client{},

		QueryToIdsStore: syncmap.NewStore(syncmap.DefaultOptions),
		TitleToIdStore:  syncmap.NewStore(syncmap.DefaultOptions),
		IdToMangaStore:  syncmap.NewStore(syncmap.DefaultOptions),

		GetClosestManga: func(title string, anilistMangas []*AnilistManga) (*AnilistManga, bool) {
			title = unifyString(title)
			return lo.MinBy(anilistMangas, func(a, b *AnilistManga) bool {
				return levenshtein.Distance(
					title,
					unifyString(a.String()),
				) < levenshtein.Distance(
					title,
					unifyString(b.String()),
				)
			}), true
		},
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
		info MangaInfo,
	) string

	// ChapterNameTemplate defines how volumes filenames will look when downloaded.
	VolumeNameTemplate func(
		provider string,
		info VolumeInfo,
	) string

	// ChapterNameTemplate defines how chapters filenames will look when downloaded.
	// E.g. "[001] chapter 1" or "Chainsaw Man - Ch. 1"
	ChapterNameTemplate func(
		provider string,
		info ChapterInfo,
	) string

	// Log is a function that will be passed to the provider
	// to serve as a progress writer
	Log LogFunc

	// Anilist is the Anilist client to use
	Anilist *Anilist
}

// DefaultClientOptions constructs default ClientOptions
func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		HTTPClient: &http.Client{},
		FS:         afero.NewOsFs(),
		ChapterNameTemplate: func(_ string, info ChapterInfo) string {
			var numStr string

			asInt, err := strconv.ParseInt(info.Number, 10, 64)
			if err == nil {
				numStr = fmt.Sprintf("%04d", asInt)
			} else {
				asFloat, err := strconv.ParseFloat(info.Number, 64)
				if err == nil {
					numStr = fmt.Sprintf("%04.1f", asFloat)
				} else {
					numStr = info.Number
				}
			}

			return sanitizePath(fmt.Sprintf("[%s] %s", numStr, info.Title))
		},
		MangaNameTemplate: func(_ string, info MangaInfo) string {
			return sanitizePath(info.Title)
		},
		VolumeNameTemplate: func(provider string, info VolumeInfo) string {
			return sanitizePath(fmt.Sprintf("Vol. %d", info.Number))
		},
		Log:     func(string) {},
		Anilist: NewAnilist(DefaultAnilistOptions()),
	}
}

// ComicInfoOptions tweaks ComicInfoXml generation
type ComicInfoOptions struct {
	// AddDate whether to add series release date or not
	AddDate bool

	// AlternativeDate use other date
	AlternativeDate *Date

	// TagRelevanceThreshold is the minimum relevance of a tag
	// to be added to ComicInfoXml.xml file. From 0 to 100
	TagRelevanceThreshold int
}

// DefaultComicInfoOptions constructs default ComicInfoOptions
func DefaultComicInfoOptions() *ComicInfoOptions {
	return &ComicInfoOptions{
		AddDate:               true,
		TagRelevanceThreshold: 60,
	}
}
