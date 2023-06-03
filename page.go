package libmangal

import (
	browser "github.com/EDDYCJY/fake-useragent"
	"io"
)

type Page struct {
	// Url is the url of the page image
	Url string

	// Data is the raw data of the page image.
	// It will have a higher priority than Url if it is not empty.
	// string is used instead of []byte because lua cannot handle []byte.
	Data string

	Headers map[string]string

	Extension string

	chapter *Chapter
}

func (p *Page) fillDefaults() {
	if p.Extension == "" {
		p.Extension = "jpg"
	}

	if p.Headers == nil {
		p.Headers = make(map[string]string)
		p.Headers["Referer"] = p.chapter.Url
		p.Headers["User-Agent"] = browser.Computer()
		p.Headers["Accept"] = "image/webp,image/apng,image/*,*/*;q=0.8"
	}
}

type downloadedPage struct {
	*Page
	io.Reader
}
