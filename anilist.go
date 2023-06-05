package libmangal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	levenshtein "github.com/ka-weihe/fast-levenshtein"
	"github.com/samber/lo"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const anilistUrlApi = "https://graphql.anilist.co"

type anilistRequestBody struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}

type Date struct {
	Year  int
	Month int
	Day   int
}

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
	// ID is the id of the manga on Anilist.
	ID int `json:"id" jsonschema:"description=ID of the manga on Anilist."`
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
	SiteURL string `json:"siteUrl" jsonschema:"description=URL of the manga on Anilist."`
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

func (c *Client) AnilistSearchManga(
	ctx context.Context,
	query string,
) ([]*AnilistManga, error) {
	query = unifyString(query)

	c.options.Log("Searching manga on Anilist...")

	{
		var ids []int
		found, err := c.options.Anilist.QueryToIdsStore.Get(query, &ids)
		if err != nil {
			_ = c.options.Anilist.QueryToIdsStore.Delete(query)
		} else if found {
			mangas := lo.FilterMap(ids, func(id, _ int) (*AnilistManga, bool) {
				var manga *AnilistManga
				found, err := c.options.Anilist.IdToMangaStore.Get(strconv.Itoa(id), &manga)

				return manga, found && err == nil
			})

			if len(mangas) == 0 {
				_ = c.options.Anilist.QueryToIdsStore.Delete(query)
				return c.AnilistSearchManga(ctx, query)
			}

			c.options.Log("Found in cache")
			return mangas, nil
		}

		c.options.Log("Not found in cache")
	}

	body := anilistRequestBody{
		Query: anilistQuerySearchByName,
		Variables: map[string]any{
			"query": query,
		},
	}

	marshalled, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, anilistUrlApi, bytes.NewReader(marshalled))
	if err != nil {
		return nil, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := c.options.HTTPClient.Do(request)
	if err != nil {
		return nil, err
	}

	defer response.Body.Close()

	// https://anilist.gitbook.io/anilist-apiv2-docs/overview/rate-limiting
	if response.StatusCode == http.StatusTooManyRequests {
		retryAfter := response.Header.Get("X-RateLimit-Remaining")
		if retryAfter == "" {
			// 90 seconds
			retryAfter = "90"
		}

		seconds, err := strconv.Atoi(retryAfter)
		if err != nil {
			return nil, err
		}

		c.options.Log(fmt.Sprintf("Rate limited. Retrying in %d seconds...", seconds))

		select {
		case <-time.After(time.Duration(seconds) * time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		return c.AnilistSearchManga(ctx, query)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(response.Status)
	}

	anilistResponse := struct {
		Data struct {
			Page struct {
				Media []*AnilistManga `json:"media"`
			} `json:"page"`
		} `json:"data"`
	}{}

	if err := json.NewDecoder(response.Body).Decode(&anilistResponse); err != nil {
		return nil, err
	}

	mangas := anilistResponse.Data.Page.Media

	c.options.Log(fmt.Sprintf("Found %d manga(s) on Anilist.", len(mangas)))

	{
		var ids = make([]int, len(mangas))

		for i, manga := range mangas {
			id := manga.ID
			ids[i] = id
			_ = c.options.Anilist.IdToMangaStore.Set(strconv.Itoa(id), manga)
		}

		_ = c.options.Anilist.QueryToIdsStore.Set(query, ids)
	}

	return mangas, nil
}

func unifyString(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func (c *Client) AnilistFindClosestManga(
	ctx context.Context,
	title string,
) (*AnilistManga, error) {
	c.options.Log("Finding closest manga on Anilist...")

	title = unifyString(title)

	{
		var id int
		found, err := c.options.Anilist.TitleToIdStore.Get(title, &id)
		if err != nil {
			_ = c.options.Anilist.TitleToIdStore.Delete(title)
		} else if found {
			var manga *AnilistManga
			found, _ = c.options.Anilist.IdToMangaStore.Get(strconv.Itoa(id), &manga)
			// TODO: handle error, maybe
			if found {
				c.options.Log("Found in cache")
				return manga, nil
			}
		}

		c.options.Log("Not found in cache")
	}

	manga, err := c.anilistFindClosestManga(ctx, title, title, 3, 0, 3)
	if err != nil {
		return nil, err
	}

	_ = c.options.Anilist.TitleToIdStore.Set(title, manga.ID)
	return manga, nil
}

func (c *Client) anilistFindClosestManga(
	ctx context.Context,
	originalTitle, currentTitle string,
	step, try, limit int,
) (*AnilistManga, error) {
	if try >= limit {
		return nil, fmt.Errorf("no results found on Anilist for manga %q", originalTitle)
	}

	c.options.Log(
		fmt.Sprintf("Finding closest manga on Anilist (try %d/%d)", try+1, limit),
	)

	mangas, err := c.AnilistSearchManga(ctx, currentTitle)
	if err != nil {
		return nil, err
	}

	if len(mangas) == 0 {
		// try again with a different title
		// remove `step` characters from the end of the title
		// avoid removing the last character or leaving an empty string
		var newLen int
		if len(currentTitle) > step {
			newLen = len(currentTitle) - step
		} else if len(currentTitle) > 1 {
			newLen = len(currentTitle) - 1
		} else {
			// trigger limit, proceeding further will only make things worse
			return c.anilistFindClosestManga(ctx, originalTitle, originalTitle, step, limit, limit)
		}

		currentTitle = currentTitle[:newLen]
		return c.anilistFindClosestManga(ctx, originalTitle, currentTitle, step, try+1, limit)
	}

	// find the closest match
	closest := lo.MinBy(mangas, func(a, b *AnilistManga) bool {
		return levenshtein.Distance(
			currentTitle,
			unifyString(a.String()),
		) < levenshtein.Distance(
			currentTitle,
			unifyString(b.String()),
		)
	})

	c.options.Log(fmt.Sprintf("Found closest manga on Anilist: %q #%d", closest.String(), closest.ID))

	return closest, nil
}
