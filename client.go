package libmangal

import (
	"github.com/samber/lo"
	"regexp"
	"strings"
)

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
