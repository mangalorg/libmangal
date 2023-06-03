package libmangal

import (
	"fmt"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
	"github.com/samber/lo"
	"github.com/spf13/afero"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type ChapterNameData struct {
	Title      string
	Number     string
	MangaTitle string
}

type MangaNameData struct {
	Title string
}

type Options struct {
	Store          gokv.Store
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

	if o.Store == nil {
		o.Store = syncmap.NewStore(syncmap.DefaultOptions)
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

type Client struct {
	options *Options
}

func sanitizePath(path string) string {
	for _, ch := range invalidPathChars {
		path = strings.ReplaceAll(path, string(ch), "_")
	}

	// replace two or more consecutive underscores with one underscore
	return regexp.MustCompile(`_+`).ReplaceAllString(path, "_")
}

func NewClient(options Options) *Client {
	options.fillDefaults()

	client := &Client{
		options: &options,
	}

	return client
}

func (c *Client) ProviderHandleFromPath(path string) ProviderHandle {
	return ProviderHandle{
		client: c,
		path:   path,
	}
}

func (c *Client) ProvidersHandles() []ProviderHandle {
	return lo.Map(c.options.ProvidersPaths, func(path string, _ int) ProviderHandle {
		return ProviderHandle{
			client: c,
			path:   path,
		}
	})
}
