package libmangal

import (
	"fmt"
	"strings"
)

type AnilistManga struct {
	// Title of the manga
	Title struct {
		// Romaji is the romanized title of the manga.
		Romaji string `json:"romaji" jsonschema:"description=Romanized title of the manga."`
		// English is the english title of the manga.
		English string `json:"english" jsonschema:"description=English title of the manga."`
		// Native is the native title of the manga. (Usually in kanji)
		Native string `json:"native" jsonschema:"description=Native title of the manga. Usually in kanji."`
	} `json:"title"`
	AverageScore int `json:"averageScore" jsonschema:"description=Average score of the manga on Anilist."`
	// ID is the id of the manga on Anilist.
	ID int `json:"id" jsonschema:"description=ID of the manga on AnilistSearch."`
	// Description is the description of the manga in html format.
	Description string `json:"description" jsonschema:"description=Description of the manga in html format."`
	// CoverImage is the cover image of the manga.
	CoverImage struct {
		// ExtraLarge is the url of the extra large cover image.
		// If the image is not available, large will be used instead.
		ExtraLarge string `json:"extraLarge" jsonschema:"description=URL of the extra large cover image. If the image is not available, large will be used instead."`
		// Large is the url of the large cover image.
		Large string `json:"large" jsonschema:"description=URL of the large cover image."`
		// Medium is the url of the medium cover image.
		Medium string `json:"medium" jsonschema:"description=URL of the medium cover image."`
		// Color is the average color of the cover image.
		Color string `json:"color" jsonschema:"description=Average color of the cover image."`
	} `json:"coverImage" jsonschema:"description=Cover image of the manga."`
	// BannerImage of the media
	BannerImage string `json:"bannerImage" jsonschema:"description=Banner image of the manga."`
	// Tags are the tags of the manga.
	Tags []struct {
		// Name of the tag.
		Name string `json:"name" jsonschema:"description=Name of the tag."`
		// Description of the tag.
		Description string `json:"description" jsonschema:"description=Description of the tag."`
		// Rank of the tag. How relevant it is to the manga from 1 to 100.
		Rank int `json:"rank" jsonschema:"description=Rank of the tag. How relevant it is to the manga from 1 to 100."`
	} `json:"tags"`
	// Genres of the manga
	Genres []string `json:"genres" jsonschema:"description=Genres of the manga."`
	// Characters are the primary characters of the manga.
	Characters struct {
		Nodes []struct {
			Name struct {
				// Full is the full name of the character.
				Full string `json:"full" jsonschema:"description=Full name of the character."`
				// Native is the native name of the character. Usually in kanji.
				Native string `json:"native" jsonschema:"description=Native name of the character. Usually in kanji."`
			} `json:"name"`
		} `json:"nodes"`
	} `json:"characters"`
	Staff struct {
		Edges []struct {
			Role string `json:"role" jsonschema:"description=Role of the staff member."`
			Node struct {
				Name struct {
					Full string `json:"full" jsonschema:"description=Full name of the staff member."`
				} `json:"name"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"staff"`
	// StartDate is the date the manga started publishing.
	StartDate Date `json:"startDate" jsonschema:"description=Date the manga started publishing."`
	// EndDate is the date the manga ended publishing.
	EndDate Date `json:"endDate" jsonschema:"description=Date the manga ended publishing."`
	// Synonyms are the synonyms of the manga (Alternative titles).
	Synonyms []string `json:"synonyms" jsonschema:"description=Synonyms of the manga (Alternative titles)."`
	// Status is the status of the manga. (FINISHED, RELEASING, NOT_YET_RELEASED, CANCELLED)
	Status string `json:"status" jsonschema:"enum=FINISHED,enum=RELEASING,enum=NOT_YET_RELEASED,enum=CANCELLED,enum=HIATUS"`
	// IDMal is the id of the manga on MyAnimeList.
	IDMal int `json:"idMal" jsonschema:"description=ID of the manga on MyAnimeList."`
	// Chapters is the amount of chapters the manga has when complete.
	Chapters int `json:"chapters" jsonschema:"description=Amount of chapters the manga has when complete."`
	// SiteURL is the url of the manga on Anilist.
	SiteURL string `json:"siteUrl" jsonschema:"description=URL of the manga on AnilistSearch."`
	// Country of origin of the manga.
	Country string `json:"countryOfOrigin" jsonschema:"description=Country of origin of the manga."`
	// External urls related to the manga.
	External []struct {
		URL string `json:"url" jsonschema:"description=URL of the external link."`
	} `json:"externalLinks" jsonschema:"description=External links related to the manga."`
}

func (a AnilistManga) String() string {
	if a.Title.English != "" {
		return a.Title.English
	}

	if a.Title.Romaji != "" {
		return a.Title.Romaji
	}

	return a.Title.Native
}

type MangaWithAnilist struct {
	Manga
	Anilist AnilistManga
}

func (m *MangaWithAnilist) SeriesJson() SeriesJson {
	var status string
	switch m.Anilist.Status {
	case "FINISHED":
		status = "Ended"
	case "RELEASING":
		status = "Continuing"
	default:
		status = "Unknown"
	}

	var publisher string
	for _, edge := range m.Anilist.Staff.Edges {
		if strings.Contains(edge.Role, "role") {
			publisher = edge.Node.Name.Full
			break
		}
	}

	publicationRun := fmt.Sprintf(
		"%d %d - %d %d",
		m.Anilist.StartDate.Month, m.Anilist.StartDate.Year,
		m.Anilist.EndDate.Month, m.Anilist.EndDate.Year,
	)

	seriesJson := SeriesJson{}
	seriesJson.Metadata.Type = "comicSeries"
	seriesJson.Metadata.Name = m.Info().Title
	seriesJson.Metadata.DescriptionFormatted = m.Anilist.Description
	seriesJson.Metadata.DescriptionText = m.Anilist.Description
	seriesJson.Metadata.Status = status
	seriesJson.Metadata.Year = m.Anilist.StartDate.Year
	seriesJson.Metadata.ComicImage = m.Anilist.CoverImage.ExtraLarge
	seriesJson.Metadata.Publisher = publisher
	seriesJson.Metadata.BookType = "Print"
	seriesJson.Metadata.TotalIssues = m.Anilist.Chapters
	seriesJson.Metadata.PublicationRun = publicationRun

	return seriesJson
}

type ChapterOfMangaWithAnilist struct {
	Chapter
	MangaWithAnilist MangaWithAnilist
}

func (c ChapterOfMangaWithAnilist) ComicInfoXml() ComicInfoXml {
	var characters = make([]string, len(c.MangaWithAnilist.Anilist.Characters.Nodes))
	for i, node := range c.MangaWithAnilist.Anilist.Characters.Nodes {
		characters[i] = node.Name.Full
	}

	date := c.MangaWithAnilist.Anilist.StartDate

	var (
		writers,
		pencillers,
		letterers,
		translators []string
	)

	for _, edge := range c.MangaWithAnilist.Anilist.Staff.Edges {
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
	for _, tag := range c.MangaWithAnilist.Anilist.Tags {
		if tag.Rank < 60 {
			continue
		}

		tags = append(tags, tag.Name)
	}

	// TODO: fill missing
	return ComicInfoXml{
		Title:           c.Info().Title,
		Series:          c.Volume().Manga().Info().Title,
		Number:          c.Info().Number,
		Web:             c.Info().URL,
		Genres:          c.MangaWithAnilist.Anilist.Genres,
		Summary:         c.MangaWithAnilist.Anilist.Description,
		Count:           c.MangaWithAnilist.Anilist.Chapters,
		Characters:      characters,
		Year:            date.Year,
		Month:           date.Month,
		Day:             date.Day,
		Publisher:       "",
		LanguageISO:     "",
		StoryArc:        "",
		StoryArcNumber:  0,
		ScanInformation: "",
		AgeRating:       "",
		CommunityRating: float32(c.MangaWithAnilist.Anilist.AverageScore) / 20,
		Review:          "",
		GTIN:            "",
		Writers:         writers,
		Format:          "",
		Pencillers:      pencillers,
		Letterers:       letterers,
		Translators:     translators,
		Tags:            tags,
		Notes:           "",
	}
}
