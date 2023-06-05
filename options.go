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
	Format         Format
	CreateMangaDir bool
	SkipIfExists   bool
}

type ReadOptions struct {
	Format Format
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
