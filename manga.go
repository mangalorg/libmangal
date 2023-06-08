package libmangal

import (
	"io"
)

//type Title struct {
//	Display string
//	Anilist string
//}
//
//type Cover struct {
//	URL     string
//	Referer string
//}
//
//type MangaInfo struct {
//	Title Title
//	URL   string
//	ID    string
//	Cover Cover
//}

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

type PageWithImage struct {
	Page
	io.Reader
}
