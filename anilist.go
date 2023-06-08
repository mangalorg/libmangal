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
	Year  int `json:"year"`
	Month int `json:"month"`
	Day   int `json:"day"`
}

type Anilist struct {
	options *AnilistOptions
}

func NewAnilist(options *AnilistOptions) *Anilist {
	return &Anilist{options: options}
}

func (a *Anilist) SearchByID(
	ctx context.Context,
	id int,
) (*AnilistManga, error) {
	a.options.Log(fmt.Sprintf("Searching manga with id %d on Anilist", id))

	{
		var manga *AnilistManga
		found, err := a.options.IdToMangaStore.Get(strconv.Itoa(id), &manga)
		if err != nil {
			_ = a.options.IdToMangaStore.Delete(strconv.Itoa(id))
		} else if found {
			return manga, nil
		}
	}

	body := &anilistRequestBody{
		Query: anilistQuerySearchByID,
		Variables: map[string]any{
			"id": id,
		},
	}

	response := struct {
		Data struct {
			Media *AnilistManga `json:"media"`
		} `json:"data"`
	}{}

	err := a.sendRequest(ctx, body, &response)
	if err != nil {
		return nil, err
	}

	manga := response.Data.Media

	_ = a.options.IdToMangaStore.Set(strconv.Itoa(id), manga)

	return manga, nil
}

func (a *Anilist) SearchMangas(
	ctx context.Context,
	query string,
) ([]*AnilistManga, error) {
	query = unifyString(query)

	a.options.Log("Searching manga on Anilist...")

	{
		var ids []int
		found, err := a.options.QueryToIdsStore.Get(query, &ids)
		if err != nil {
			_ = a.options.QueryToIdsStore.Delete(query)
		} else if found {
			mangas := lo.FilterMap(ids, func(id, _ int) (*AnilistManga, bool) {
				var manga *AnilistManga
				found, err := a.options.IdToMangaStore.Get(strconv.Itoa(id), &manga)

				return manga, found && err == nil
			})

			a.options.Log("Found in cache")
			return mangas, nil
		}

		a.options.Log("Not found in cache")
	}

	body := &anilistRequestBody{
		Query: anilistQuerySearchByName,
		Variables: map[string]any{
			"query": query,
		},
	}

	anilistResponse := struct {
		Data struct {
			Page struct {
				Media []*AnilistManga `json:"media"`
			} `json:"page"`
		} `json:"data"`
	}{}

	err := a.sendRequest(ctx, body, &anilistResponse)
	if err != nil {
		return nil, err
	}

	mangas := anilistResponse.Data.Page.Media

	a.options.Log(fmt.Sprintf("Found %d manga(s) on Anilist.", len(mangas)))

	{
		var ids = make([]int, len(mangas))

		for i, manga := range mangas {
			id := manga.ID
			ids[i] = id
			_ = a.options.IdToMangaStore.Set(strconv.Itoa(id), manga)
		}

		_ = a.options.QueryToIdsStore.Set(query, ids)
	}

	return mangas, nil
}

func (a *Anilist) sendRequest(
	ctx context.Context,
	body *anilistRequestBody,
	decode any,
) error {
	marshalled, err := json.Marshal(body)
	if err != nil {
		return err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, anilistUrlApi, bytes.NewReader(marshalled))
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := a.options.HTTPClient.Do(request)
	if err != nil {
		return err
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
			return err
		}

		a.options.Log(fmt.Sprintf("Rate limited. Retrying in %d seconds...", seconds))

		select {
		case <-time.After(time.Duration(seconds) * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}

		return a.sendRequest(ctx, body, decode)
	}

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf(response.Status)
	}

	return json.NewDecoder(response.Body).Decode(&decode)
}

func (a *Anilist) FindClosestManga(
	ctx context.Context,
	title string,
) (*AnilistManga, error) {
	a.options.Log("Finding closest manga on Anilist...")

	title = unifyString(title)

	{
		var id int
		found, err := a.options.TitleToIdStore.Get(title, &id)
		if err != nil {
			_ = a.options.TitleToIdStore.Delete(title)
		} else if found {
			var manga *AnilistManga
			found, _ = a.options.IdToMangaStore.Get(strconv.Itoa(id), &manga)
			// TODO: handle error, maybe
			if found {
				a.options.Log("Found in cache")
				return manga, nil
			} else {
				manga, err = a.SearchByID(ctx, id)
				if err != nil {
					return nil, err
				}

				return manga, nil
			}
		}

		a.options.Log("Not found in cache")
	}

	manga, err := a.findClosestManga(ctx, title, title, 3, 0, 3)
	if err != nil {
		return nil, err
	}

	_ = a.options.TitleToIdStore.Set(title, manga.ID)
	return manga, nil
}

func (a *Anilist) findClosestManga(
	ctx context.Context,
	originalTitle, currentTitle string,
	step, try, limit int,
) (*AnilistManga, error) {
	if try >= limit {
		return nil, fmt.Errorf("no results found on Anilist for manga %q", originalTitle)
	}

	a.options.Log(
		fmt.Sprintf("Finding closest manga on Anilist (try %d/%d)", try+1, limit),
	)

	mangas, err := a.SearchMangas(ctx, currentTitle)
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
			return a.findClosestManga(ctx, originalTitle, originalTitle, step, limit, limit)
		}

		currentTitle = currentTitle[:newLen]
		return a.findClosestManga(ctx, originalTitle, currentTitle, step, try+1, limit)
	}

	// find the closest match
	closest, ok := a.options.GetClosestManga(currentTitle, mangas)

	if !ok {
		return nil, fmt.Errorf("closest manga to %q wasn't found on anilist", originalTitle)
	}

	a.options.Log(fmt.Sprintf("Found closest manga on Anilist: %q #%d", closest.String(), closest.ID))
	return closest, nil
}

func (a *Anilist) BindTitleWithId(title string, anilistMangaId int) error {
	return a.options.TitleToIdStore.Set(title, anilistMangaId)
}

func (a *Anilist) MakeMangaWithAnilist(ctx context.Context, manga Manga) (*MangaWithAnilist, error) {
	anilistManga, err := a.FindClosestManga(ctx, manga.GetTitle())
	if err != nil {
		return nil, err
	}

	return &MangaWithAnilist{
		Manga:   manga,
		Anilist: anilistManga,
	}, nil
}

func (a *Anilist) MakeChapterWithAnilist(ctx context.Context, chapter Chapter) (*ChapterOfMangaWithAnilist, error) {
	mangaWithAnilist, err := a.MakeMangaWithAnilist(ctx, chapter.GetManga())
	if err != nil {
		return nil, err
	}

	return &ChapterOfMangaWithAnilist{
		Chapter:          chapter,
		MangaWithAnilist: mangaWithAnilist,
	}, nil
}
