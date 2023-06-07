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

type DownloadOptions struct {
	Format       Format
	SkipIfExists bool

	DownloadMangaCover bool
	WriteSeriesJson    bool
	WriteComicInfoXml  bool
	ComicInfoOptions   *ComicInfoOptions
}

func DefaultDownloadOptions() *DownloadOptions {
	return &DownloadOptions{
		Format:       FormatPDF,
		SkipIfExists: true,

		DownloadMangaCover: false,
		WriteSeriesJson:    false,
		WriteComicInfoXml:  false,
		ComicInfoOptions:   DefaultComicInfoOptions(),
	}
}

type ReadOptions struct {
	Format Format

	// MangasLibraryPath is the path to the directory where mangas are stored.
	// Will be used to see if the given chapter is already downloaded,
	// so it will be opened instead
	MangasLibraryPath string
}

func DefaultReadOptions() *ReadOptions {
	return &ReadOptions{
		Format:            FormatPDF,
		MangasLibraryPath: "",
	}
}

type AnilistOptions struct {
	QueryToIdsStore gokv.Store
	TitleToIdStore  gokv.Store
	IdToMangaStore  gokv.Store

	// GetClosestManga is the function that will be used
	// to get the most similar manga from Anilist
	// based on the passed title.
	// If there's no such manga ok will be false
	GetClosestManga func(
		title string,
		anilistMangas []*AnilistManga,
	) (closest *AnilistManga, ok bool)
}

func DefaultAnilistOptions() *AnilistOptions {
	return &AnilistOptions{
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

type ClientOptions struct {
	HTTPClient *http.Client
	FS         afero.Fs

	ChapterNameTemplate func(
		provider string,
		data ChapterNameData,
	) string

	MangaNameTemplate func(
		provider string,
		data MangaNameData,
	) string

	Log func(string)

	Anilist *AnilistOptions
}

func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		HTTPClient: &http.Client{},
		FS:         afero.NewOsFs(),
		ChapterNameTemplate: func(_ string, data ChapterNameData) string {
			var numStr string

			asInt, err := strconv.ParseInt(data.Number, 10, 64)
			if err == nil {
				numStr = fmt.Sprintf("%04d", asInt)
			} else {
				asFloat, err := strconv.ParseFloat(data.Number, 64)
				if err == nil {
					numStr = fmt.Sprintf("%04.1f", asFloat)
				} else {
					numStr = data.Number
				}
			}

			return sanitizePath(fmt.Sprintf("[%s] %s", numStr, data.Title))
		},
		MangaNameTemplate: func(_ string, data MangaNameData) string {
			return sanitizePath(data.Title)
		},
		Log:     func(string) {},
		Anilist: DefaultAnilistOptions(),
	}
}

type ComicInfoOptions struct {
	// AddDate whether to add series release date or not
	AddDate bool

	// AlternativeDate use other date
	AlternativeDate *Date

	// TagRelevanceThreshold is the minimum relevance of a tag
	// to be added to ComicInfoXml.xml file. From 0 to 100
	TagRelevanceThreshold int
}

func DefaultComicInfoOptions() *ComicInfoOptions {
	return &ComicInfoOptions{
		AddDate:               true,
		TagRelevanceThreshold: 60,
	}
}
