package libmangal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/samber/lo"
	"net/http"
	"strconv"
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
	closest, ok := c.options.Anilist.GetClosestManga(currentTitle, mangas)

	if !ok {
		return nil, fmt.Errorf("closest manga to %q wasn't found on anilist", originalTitle)
	}

	c.options.Log(fmt.Sprintf("Found closest manga on Anilist: %q #%d", closest.String(), closest.ID))
	return closest, nil
}

func (c *Client) BindTitleWithAnilistId(title string, anilistMangaId int) error {
	return c.options.Anilist.TitleToIdStore.Set(title, anilistMangaId)
}

func (c *Client) MakeMangaWithAnilist(ctx context.Context, manga *Manga) (*MangaWithAnilist, error) {
	anilistManga, err := c.AnilistFindClosestManga(ctx, manga.Title)
	if err != nil {
		return nil, err
	}

	return &MangaWithAnilist{
		Manga:        manga,
		AnilistManga: anilistManga,
	}, nil
}

func (c *Client) MakeChapterWithAnilist(ctx context.Context, chapter *Chapter) (*ChapterOfMangaWithAnilist, error) {
	mangaWithAnilist, err := c.MakeMangaWithAnilist(ctx, chapter.manga)
	if err != nil {
		return nil, err
	}

	return &ChapterOfMangaWithAnilist{
		Chapter:          chapter,
		MangaWithAnilist: mangaWithAnilist,
	}, nil
}
