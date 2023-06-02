package libmangal

import (
	"context"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
	"github.com/samber/lo"
	"github.com/spf13/afero"
	"net/http"
)

type Options struct {
	Store          *gokv.Store
	HTTPClient     *http.Client
	FS             *afero.Fs
	ProvidersPaths []string

	// TODO: add anilist
}

type Client struct {
	store         gokv.Store
	httpClient    *http.Client
	fs            afero.Fs
	providerPaths []string

	context       context.Context
	contextCancel context.CancelFunc
}

func NewClient(options Options) *Client {
	client := &Client{}

	if options.HTTPClient != nil {
		client.httpClient = options.HTTPClient
	} else {
		client.httpClient = http.DefaultClient
	}

	if options.Store != nil {
		client.store = *options.Store
	} else {
		client.store = syncmap.NewStore(syncmap.DefaultOptions)
	}

	if options.FS != nil {
		client.fs = *options.FS
	} else {
		client.fs = afero.NewOsFs()
	}

	client.providerPaths = options.ProvidersPaths

	return client
}

func (c *Client) Cancel() {
	c.contextCancel()
	c.context, c.contextCancel = context.WithCancel(context.Background())
}

func (c *Client) ProviderHandleFromPath(path string) ProviderHandle {
	return ProviderHandle{
		client: c,
		path:   path,
	}
}

func (c *Client) ProvidersHandles() []ProviderHandle {
	return lo.Map(c.providerPaths, func(path string, _ int) ProviderHandle {
		return ProviderHandle{
			client: c,
			path:   path,
		}
	})
}
