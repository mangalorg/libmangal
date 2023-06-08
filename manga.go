package libmangal

import (
	"io"
)

type MangaInfo struct {
	Title   string
	Anilist string
	URL     string
	ID      string
	Cover   string
}

type Manga interface {
	Info() MangaInfo
}

type VolumeInfo struct {
	Number    int
	MangaInfo func() MangaInfo
}

type Volume interface {
	Info() VolumeInfo
}

type ChapterInfo struct {
	Title      string
	URL        string
	Number     string
	VolumeInfo func() VolumeInfo
}

type Chapter interface {
	Info() ChapterInfo
}

type Page interface {
	GetExtension() string
}

type PageWithImage[P Page] struct {
	Page  P
	Image io.Reader
}
