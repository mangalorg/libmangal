package libmangal

import (
	"io"
	"net/http"
	"strings"
)

type Page struct {
	// Url is the url of the page image
	Url string

	// Data is the raw data of the page image.
	// It will have a higher priority than Url if it is not empty.
	// string is used instead of []byte because lua cannot handle []byte.
	Data string

	Headers map[string]string
}

func (p *Page) Reader(provider *Provider) (io.Reader, error) {
	if p.Data != "" {
		return strings.NewReader(p.Data), nil
	}

	request, _ := http.NewRequestWithContext(provider.client.context, http.MethodGet, p.Url, nil)

	if p.Headers != nil {
		for key, value := range p.Headers {
			request.Header.Set(key, value)
		}
	}

	response, err := provider.client.httpClient.Do(request)
	if err != nil {
		return nil, err
	}

	return response.Body, nil
}
