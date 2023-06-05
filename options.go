package libmangal

import (
	"fmt"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
	"github.com/spf13/afero"
	"net/http"
	"strconv"
)

type DownloadOptions struct {
	Format            Format
	CreateMangaDir    bool
	SkipIfExists      bool
	WriteSeriesJson   bool
	WriteComicInfoXml bool
	ComicInfoOptions  *ComicInfoOptions
}

func DefaultDownloadOptions() *DownloadOptions {
	return &DownloadOptions{
		Format:            FormatPDF,
		CreateMangaDir:    true,
		SkipIfExists:      true,
		WriteSeriesJson:   false,
		WriteComicInfoXml: false,
		ComicInfoOptions:  DefaultComicInfoOptions(),
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
}

func DefaultAnilistOptions() *AnilistOptions {
	return &AnilistOptions{
		QueryToIdsStore: syncmap.NewStore(syncmap.DefaultOptions),
		TitleToIdStore:  syncmap.NewStore(syncmap.DefaultOptions),
		IdToMangaStore:  syncmap.NewStore(syncmap.DefaultOptions),
	}
}

type ClientOptions struct {
	HTTPClient *http.Client
	FS         afero.Fs

	ChapterNameTemplate func(ChapterNameData) string
	MangaNameTemplate   func(MangaNameData) string

	Log func(string)

	Anilist *AnilistOptions
}

func DefaultClientOptions() *ClientOptions {
	return &ClientOptions{
		HTTPClient: &http.Client{},
		FS:         afero.NewOsFs(),
		ChapterNameTemplate: func(data ChapterNameData) string {
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
		MangaNameTemplate: func(data MangaNameData) string {
			return sanitizePath(data.Title)
		},
		Log:     func(string) {},
		Anilist: DefaultAnilistOptions(),
	}
}

type ComicInfoOptions struct {
	AddDate         bool
	AlternativeDate *Date

	// TagRelevanceThreshold is the minimum relevance of a tag
	// to be added to ComicInfo.xml file. From 0 to 100
	TagRelevanceThreshold int
}

func DefaultComicInfoOptions() *ComicInfoOptions {
	return &ComicInfoOptions{
		AddDate:               true,
		TagRelevanceThreshold: 60,
	}
}
