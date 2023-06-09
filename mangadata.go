package libmangal

import (
	"io"
)

type MangaInfo struct {
	// Title of the manga
	Title string

	// AnilistSearch is the title of the manga
	// that will be used for on Anilist.
	//
	// This is a separate field from the Title due to Title could
	// be on any language, but Anilist only supports searching
	// for english, native and romaji titles.
	AnilistSearch string

	// URL leading to manga page web page.
	URL string

	// ID of the Manga. It must be unique withing its provider.
	ID string

	// Cover is the cover image url.
	Cover string
}

type Manga interface {
	Info() MangaInfo

	// SeriesJson will be used to write series.json file.
	// If ok is false then mangal will try to search on Anilist for the
	// relevant manga.
	SeriesJson() (seriesJson SeriesJson, ok bool)
}

type VolumeInfo struct {
	// Number of the volume. Must be greater than 0
	Number int
}

// Volume if a series is popular enough, its chapters
// are then collected and published into volumes,
// which usually feature a few chapters of the overall story.
// Most Manga series are long-running and can span multiple volumes.
//
// Mangal expects that each Manga must have at least one Volume
type Volume interface {
	Info() VolumeInfo

	// Manga gets the Manga that this Volume is relevant to.
	//
	// Implementation should not make any external requests
	// nor be computationally heavy.
	Manga() Manga
}

type ChapterInfo struct {
	// Title is the title of chapter
	Title string

	// URL is the url leading to chapter web page.
	URL string

	// Number of the chapter.
	//
	// Float type used in case of chapters that has numbers
	// like this: 10.8 or 103.1.
	Number float32
}

// Chapter is what Volume consists of. Each chapter is about 24–40 pages.
type Chapter interface {
	Info() ChapterInfo

	// Volume gets the Volume that this Chapter is relevant to.
	//
	// Implementation should not make any external requests
	// nor be computationally heavy.
	Volume() Volume

	// ComicInfoXml will be used to write ComicInfo.xml file.
	// If ok is false then mangal will try to search on Anilist for the
	// relevant manga.
	ComicInfoXml(options ComicInfoXmlOptions) (comicInfo ComicInfoXml, ok bool)
}

// Page is what Chapter consists of.
type Page interface {
	// GetExtension gets the image extension of this page.
	// An extension must start with the dot.
	//
	// For example: .jpeg .png
	GetExtension() string

	// Chapter gets the Chapter that this Page is relevant to.
	//
	// Implementation should not make any external requests
	// nor be computationally heavy.
	Chapter() Chapter
}

// PageWithImage is a Page with downloaded image
type PageWithImage[P Page] struct {
	Page  P
	Image io.Reader
}
