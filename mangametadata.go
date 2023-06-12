package libmangal

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
)

const (
	comicInfoXmlFilename = "ComicInfo.xml"
	seriesJsonFilename   = "series.json"
	coverJpgFilename     = "cover.jpg"
	bannerJpgFilename    = "banner.jpg"
)

// ComicInfoXml contains metadata information about a comic book.
// It is often used by comic book readers and management software
// to organize and display information about comic books in a library or collection.
type ComicInfoXml struct {
	// Title of the book
	Title string
	// Series title of the series the book is part of.
	Series string
	// Number of the book in the series.
	Number float32
	// Web a URL pointing to a reference website for the book.
	Web string

	// Genres of the book or series. For example, Science-Fiction or Shonen.
	Genres []string

	// Summary a description or summary of the book.
	Summary string

	// Count the total number of books in the series.
	Count int

	// Characters present in the book.
	Characters []string

	// Year of the book release
	Year int

	// Month of the book release
	Month int

	// Day of the book release
	Day int

	// Publisher person or organization responsible for
	// publishing, releasing, or issuing a resource.
	Publisher string

	// LanguageISO A language code describing the language of the book.
	LanguageISO string

	// StoryArc the story arc that books belong to.
	StoryArc string

	// StoryArcNumber While StoryArc was originally designed to store the arc within a series,
	// it was often used to indicate that a book was part of a reading order, composed of books
	// from multiple series. Mylar for instance was using the field as such.
	//
	// Since StoryArc itself wasn't able to carry the information about ordering of books
	// within a reading order, StoryArcNumber was added.
	//
	// StoryArc and StoryArcNumber can work in combination, to indicate in
	// which position the book is located at for a specific reading order.
	StoryArcNumber int

	// ScanInformation is a free text field, usually used to store information about who
	// scanned the book.
	ScanInformation string

	// AgeRating of the book.
	AgeRating string

	// CommunityRating Community rating of the book, from 0.0 to 5.0.
	CommunityRating float32

	// Review of the book.
	Review string

	// GTIN a Global Trade Item Number identifying the book.
	// GTIN incorporates other standards like ISBN, ISSN, EAN, or JAN.
	//
	// https://en.wikipedia.org/wiki/Global_Trade_Item_Number
	GTIN string

	// Writers people or organizations responsible for creating the scenario.
	Writers []string

	// Format the original publication's binding format for scanned physical books or presentation format for digital sources.
	//
	// "TBP", "HC", "Web", "Digital" are common designators.
	Format string

	// Pencillers people or organizations responsible for drawing the art.
	Pencillers []string

	// Letterers people or organizations responsible for drawing text and speech bubbles.
	Letterers []string

	// Translators people or organizations responsible for rendering a text from one language into another,
	// or from an older form of a language into the modern form.
	//
	// This can also be used for fan translations ("scanlator").
	Translators []string

	// Tags of the book or series. For example, ninja or school life.
	Tags []string

	// Notes a free text field, usually used to store information about
	// the application that created the ComicInfo.xml file.
	Notes string
}

func (c ComicInfoXml) wrapper(options ComicInfoXmlOptions) comicInfoXmlWrapper {
	wrapper := comicInfoXmlWrapper{
		XmlnsXsd:   "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi:   "http://www.w3.org/2001/XMLSchema-instance",
		Title:      c.Title,
		Series:     c.Series,
		Number:     c.Number,
		Web:        c.Web,
		Genre:      strings.Join(c.Genres, ","),
		Summary:    c.Summary,
		Count:      c.Count,
		Characters: strings.Join(c.Characters, ","),
		Year:       c.Year,
		Month:      c.Month,
		Day:        c.Day,
		Writer:     strings.Join(c.Writers, ","),
		Penciller:  strings.Join(c.Pencillers, ","),
		Letterer:   strings.Join(c.Letterers, ","),
		Translator: strings.Join(c.Translators, ","),
		Tags:       strings.Join(c.Tags, ","),
		Notes: strings.Join([]string{
			c.Notes,
			"",
			fmt.Sprintf("Downloaded with libmangal/%s", Version),
			"https://github.com/mangalorg/libmangal",
		}, "\n"),
		Manga:           "YesAndRightToLeft",
		StoryArc:        c.StoryArc,
		StoryArcNumber:  c.StoryArcNumber,
		ScanInformation: c.ScanInformation,
		AgeRating:       c.AgeRating,
		CommunityRating: c.CommunityRating,
		Review:          c.Review,
		GTIN:            c.GTIN,
		Format:          c.Format,
		LanguageISO:     c.LanguageISO,
		Publisher:       c.Publisher,
	}

	if !options.AddDate {
		wrapper.Year = 0
		wrapper.Month = 0
		wrapper.Day = 0
	} else if options.AlternativeDate != nil {
		date := options.AlternativeDate
		wrapper.Year = date.Year
		wrapper.Month = date.Month
		wrapper.Day = date.Day
	}

	return wrapper
}

type comicInfoXmlWrapper struct {
	// XMLName is a meta field that must be left unchanged
	XMLName xml.Name `xml:"ComicInfo"`
	// XmlnsXsi is a meta field that must be left unchanged
	XmlnsXsi string `xml:"xmlns:xsi,attr"`
	// XmlnsXsd is a meta field that must be left unchanged.
	XmlnsXsd string `xml:"xmlns:xsd,attr"`

	Title           string  `xml:"Title,omitempty"`
	Series          string  `xml:"Series,omitempty"`
	Number          float32 `xml:"Number,omitempty"`
	Web             string  `xml:"Web,omitempty"`
	Genre           string  `xml:"Genre,omitempty"`
	Summary         string  `xml:"Summary,omitempty"`
	Count           int     `xml:"Count,omitempty"`
	Characters      string  `xml:"Characters,omitempty"`
	PageCount       int     `xml:"PageCount,omitempty"`
	Year            int     `xml:"Year,omitempty"`
	Month           int     `xml:"Month,omitempty"`
	Day             int     `xml:"Day,omitempty"`
	Writer          string  `xml:"Writer,omitempty"`
	Penciller       string  `xml:"Penciller,omitempty"`
	Letterer        string  `xml:"Letterer,omitempty"`
	Translator      string  `xml:"Translator,omitempty"`
	Tags            string  `xml:"Tags,omitempty"`
	Notes           string  `xml:"Notes,omitempty"`
	Manga           string  `xml:"Manga,omitempty"`
	StoryArc        string  `xml:"StoryArc,omitempty"`
	StoryArcNumber  int     `xml:"StoryArcNumber,omitempty"`
	ScanInformation string  `xml:"ScanInformation,omitempty"`
	AgeRating       string  `xml:"AgeRating,omitempty"`
	CommunityRating float32 `xml:"CommunityRating,omitempty"`
	Review          string  `xml:"Review,omitempty"`
	GTIN            string  `xml:"GTIN,omitempty"`
	Format          string  `xml:"Format,omitempty"`
	LanguageISO     string  `xml:"LanguageISO,omitempty"`
	Publisher       string  `xml:"Publisher,omitempty"`
}

func (c comicInfoXmlWrapper) marshal() ([]byte, error) {
	return xml.MarshalIndent(
		c,
		"",
		"  ",
	)
}

// SeriesJson is similar to ComicInfoXml but designed for
// the series as a whole rather than a single chapter
type SeriesJson struct {
	Type                 string `json:"type"`
	Name                 string `json:"name"`
	DescriptionFormatted string `json:"description_formatted"`
	DescriptionText      string `json:"description_text"`
	Status               string `json:"status"`
	Year                 int    `json:"year"`
	ComicImage           string `json:"ComicImage"`
	Publisher            string `json:"publisher"`
	ComicID              int    `json:"comicId"`
	BookType             string `json:"booktype"`
	TotalIssues          int    `json:"total_issues"`
	PublicationRun       string `json:"publication_run"`
}

func (s SeriesJson) wrapper() seriesJsonWrapper {
	return seriesJsonWrapper{Metadata: s}
}

type seriesJsonWrapper struct {
	Metadata SeriesJson `json:"metadata"`
}

func (s seriesJsonWrapper) marshal() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
