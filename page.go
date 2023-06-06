package libmangal

import (
	"fmt"
	"github.com/pkg/errors"
	"io"
	"net/url"
	"regexp"
)

var fileExtensionRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.]*[a-zA-Z0-9]$`)

func errPage(err error) error {
	return errors.Wrap(err, "page")
}

type Page struct {
	// Url is the url of the page image
	Url string

	// Data is the raw data of the page image.
	// It will have a higher priority than Url if it is not empty.
	// string is used instead of []byte because lua cannot handle []byte.
	Data string

	Headers, Cookies map[string]string

	Extension string

	chapter *Chapter
}

func (p *Page) String() string {
	if p.Url != "" {
		return p.Url
	}

	return "<BINARY>"
}

func (p *Page) validate() error {
	if p.Url == "" && p.Data == "" {
		return errPage(fmt.Errorf("either page url or data must be non empty"))
	}

	if p.Url != "" {
		if _, err := url.Parse(p.Url); err != nil {
			return errPage(err)
		}
	}

	if p.Extension != "" {
		if !fileExtensionRegex.MatchString(p.Extension) {
			return errPage(fmt.Errorf("invalid extension: %s", p.Extension))
		}
	}

	return nil
}

func (p *Page) fillDefaults() {
	if p.Extension == "" {
		p.Extension = "jpg"
	}

	if p.Headers == nil {
		p.Headers = make(map[string]string)
		p.Headers["Referer"] = p.chapter.Url
		// TODO: generate random user-agent
		p.Headers["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/114.0.0.0 Safari/537.36"
		p.Headers["Accept"] = "image/webp,image/apng,image/*,*/*;q=0.8"
	}
}

type downloadedPage struct {
	*Page
	io.Reader
}
