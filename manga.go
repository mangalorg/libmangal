package libmangal

import (
	"io"
)

type Manga interface {
	GetTitle() string
	GetURL() string
	GetID() string
	GetCoverURL() string
}

type MangaNameData struct {
	Title string
	Id    string
}

type Chapter interface {
	GetTitle() string
	GetURL() string
	GetNumber() string
	GetManga() Manga
}

type ChapterNameData struct {
	Title      string
	Number     string
	MangaTitle string
}

type Page interface {
	GetExtension() string
}

type DownloadedPage struct {
	Page
	io.Reader
}
