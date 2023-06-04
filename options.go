package libmangal

import (
	"fmt"
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
	// TODO
	SaveHistory bool
}

type Options struct {
	HTTPClient     *http.Client
	FS             afero.Fs
	ProvidersPaths []string

	ChapterNameTemplate func(ChapterNameData) string
	MangaNameTemplate   func(MangaNameData) string

	Progress func(string)

	// TODO: add anilist options
}

func (o *Options) fillDefaults() {
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{}
	}

	if o.FS == nil {
		o.FS = afero.NewOsFs()
	}

	if o.ChapterNameTemplate == nil {
		o.ChapterNameTemplate = func(data ChapterNameData) string {
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
		}
	}

	if o.MangaNameTemplate == nil {
		o.MangaNameTemplate = func(data MangaNameData) string {
			return sanitizePath(data.Title)
		}
	}

	if o.Progress == nil {
		o.Progress = func(string) {}
	}
}
