package libmangal

import (
	"encoding/xml"
	"fmt"
	"strings"
)

const (
	comicInfoXmlFilename = "ComicInfo.xml"
	seriesJsonFilename   = "series.json"
)

// ComicInfoXml contains metadata information about a comic book.
// It is often used by comic book readers and management software
// to organize and display information about comic books in a library or collection.
type ComicInfoXml struct {
	XMLName  xml.Name `xml:"ComicInfo"`
	XmlnsXsi string   `xml:"xmlns:xsi,attr"`
	XmlnsXsd string   `xml:"xmlns:xsd,attr"`

	Title      string `xml:"Title,omitempty"`
	Series     string `xml:"Series,omitempty"`
	Number     string `xml:"Number,omitempty"`
	Web        string `xml:"Web,omitempty"`
	Genre      string `xml:"Genre,omitempty"`
	PageCount  int    `xml:"PageCount,omitempty"`
	Summary    string `xml:"Summary,omitempty"`
	Count      int    `xml:"Count,omitempty"`
	Characters string `xml:"Characters,omitempty"`
	Year       int    `xml:"Year,omitempty"`
	Month      int    `xml:"Month,omitempty"`
	Day        int    `xml:"Day,omitempty"`
	Writer     string `xml:"Writer,omitempty"`
	Penciller  string `xml:"Penciller,omitempty"`
	Letterer   string `xml:"Letterer,omitempty"`
	Translator string `xml:"Translator,omitempty"`
	Tags       string `xml:"Tags,omitempty"`
	Notes      string `xml:"Notes,omitempty"`
	Manga      string `xml:"Manga,omitempty"`
}

// SeriesJson is similar to ComicInfoXml but designed for
// the series as a whole rather than a single chapter
type SeriesJson struct {
	Metadata struct {
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
	} `json:"metadata"`
}

type MangaWithAnilist struct {
	*Manga
	*AnilistManga
}

func (m *MangaWithAnilist) SeriesJson() *SeriesJson {
	var status string
	switch m.Status {
	case "FINISHED":
		status = "Ended"
	case "RELEASING":
		status = "Continuing"
	default:
		status = "Unknown"
	}

	var publisher string
	for _, edge := range m.Staff.Edges {
		if strings.Contains(edge.Role, "role") {
			publisher = edge.Node.Name.Full
			break
		}
	}

	publicationRun := fmt.Sprintf("%d %d - %d %d", m.StartDate.Month, m.StartDate.Year, m.EndDate.Month, m.EndDate.Year)

	seriesJson := SeriesJson{}
	seriesJson.Metadata.Type = "comicSeries"
	seriesJson.Metadata.Name = m.Manga.Title
	seriesJson.Metadata.DescriptionFormatted = m.Description
	seriesJson.Metadata.DescriptionText = m.Description
	seriesJson.Metadata.Status = status
	seriesJson.Metadata.Year = m.StartDate.Year
	seriesJson.Metadata.ComicImage = m.CoverImage.ExtraLarge
	seriesJson.Metadata.Publisher = publisher
	seriesJson.Metadata.BookType = "Print"
	seriesJson.Metadata.TotalIssues = m.Chapters
	seriesJson.Metadata.PublicationRun = publicationRun

	return &seriesJson
}

type ChapterOfMangaWithAnilist struct {
	*Chapter
	*MangaWithAnilist
}

func (c *ChapterOfMangaWithAnilist) ComicInfoXml(options *ComicInfoOptions) *ComicInfoXml {
	var characters = make([]string, len(c.MangaWithAnilist.Characters.Nodes))
	for i, node := range c.MangaWithAnilist.Characters.Nodes {
		characters[i] = node.Name.Full
	}

	var date Date
	if options.AddDate {
		if options.AlternativeDate != nil {
			date = *options.AlternativeDate
		} else {
			date = c.MangaWithAnilist.StartDate
		}
	}

	var (
		writers,
		pencillers,
		letterers,
		translators []string
	)

	for _, edge := range c.AnilistManga.Staff.Edges {
		role := edge.Role
		name := edge.Node.Name.Full
		switch {
		case strings.Contains(role, "story"):
			writers = append(writers, name)
		case strings.Contains(role, "art"):
			pencillers = append(pencillers, name)
		case strings.Contains(role, "translator"):
			translators = append(translators, name)
		case strings.Contains(role, "lettering"):
			letterers = append(letterers, name)
		}
	}

	var tags = make([]string, 0)
	for _, tag := range c.MangaWithAnilist.Tags {
		if tag.Rank < options.TagRelevanceThreshold {
			continue
		}

		tags = append(tags, tag.Name)
	}

	return &ComicInfoXml{
		XmlnsXsd:   "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi:   "http://www.w3.org/2001/XMLSchema-instance",
		Title:      c.Title,
		Series:     c.Manga.Title,
		Number:     c.Number,
		Web:        c.Url,
		Genre:      strings.Join(c.MangaWithAnilist.Genres, ","),
		PageCount:  0,
		Summary:    c.MangaWithAnilist.Description,
		Count:      c.MangaWithAnilist.Chapters,
		Characters: strings.Join(characters, ","),
		Year:       date.Year,
		Month:      date.Month,
		Day:        date.Day,
		Writer:     strings.Join(writers, ","),
		Penciller:  strings.Join(pencillers, ","),
		Letterer:   strings.Join(letterers, ","),
		Translator: strings.Join(translators, ","),
		Tags:       strings.Join(tags, ","),
		Notes:      fmt.Sprintf("Downloaded with libmangal %s. https://github.com/mangalorg/libmangal", Version),
		Manga:      "YesAndRightToLeft",
	}
}