package libmangal

import "fmt"

type MangaInfo struct {
	// Title of the manga
	Title string `json:"title"`

	// AnilistSearch is the title of the manga
	// that will be used for on Anilist.
	//
	// This is a separate field from the Title due to Title could
	// be on any language, but Anilist only supports searching
	// for english, native and romaji titles.
	AnilistSearch string `json:"anilistSearch"`

	// URL leading to manga page web page.
	URL string `json:"url"`

	// ID of the Manga. It must be unique withing its provider.
	ID string `json:"id"`

	// Cover is the cover image url.
	Cover string `json:"cover"`

	// Banner is the banner image url.
	Banner string `json:"banner"`
}

type Manga interface {
	fmt.Stringer

	Info() MangaInfo
}

type MangaWithSeriesJSON interface {
	Manga

	// SeriesJSON will be used to write series.json file.
	// If ok is false then mangal will try to search on Anilist for the
	// relevant manga.
	SeriesJSON() (SeriesJSON, error)
}

type VolumeInfo struct {
	// Number of the volume. Must be greater than 0
	Number int `json:"number"`
}

// Volume if a series is popular enough, its chapters
// are then collected and published into volumes,
// which usually feature a few chapters of the overall story.
// Most Manga series are long-running and can span multiple volumes.
//
// Mangal expects that each Manga must have at least one Volume
type Volume interface {
	fmt.Stringer

	Info() VolumeInfo

	// Manga gets the Manga that this Volume is relevant to.
	//
	// Implementation should not make any external requests
	// nor be computationally heavy.
	Manga() Manga
}

type ChapterInfo struct {
	// Title is the title of chapter
	Title string `json:"title"`

	// URL is the url leading to chapter web page.
	URL string `json:"url"`

	// Number of the chapter.
	//
	// Float type used in case of chapters that has numbers
	// like this: 10.8 or 103.1.
	Number float32 `json:"number"`
}

// Chapter is what Volume consists of. Each chapter is about 24–40 pages.
type Chapter interface {
	fmt.Stringer

	Info() ChapterInfo

	// Volume gets the Volume that this Chapter is relevant to.
	//
	// Implementation should not make any external requests
	// nor be computationally heavy.
	Volume() Volume
}

type ChapterWithComicInfoXML interface {
	Chapter

	// ComicInfoXML will be used to write ComicInfo.xml file.
	// If ok is false then mangal will try to search on Anilist for the
	// relevant manga.
	ComicInfoXML() (ComicInfoXML, error)
}

// Page is what Chapter consists of.
type Page interface {
	fmt.Stringer

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
type PageWithImage interface {
	Page

	// GetImage gets the image contents. This operation should not perform any extra requests.
	// Implementation should expose this method only if the Page already contains image contents.
	GetImage() []byte

	// SetImage sets the image contents. This is used by DownloadOptions.ImageTransformer
	SetImage(newImage []byte)
}

type pageWithImage struct {
	Page
	image []byte
}

func (p *pageWithImage) GetImage() []byte {
	return p.image
}

func (p *pageWithImage) SetImage(newImage []byte) {
	p.image = newImage
}
