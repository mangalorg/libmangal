package libmangal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func (a *Anilist) GetByID(
	ctx context.Context,
	id int,
) (*AnilistManga, error) {
	found, manga, err := a.cacheStatusId(id)
	if err != nil {
		return nil, err
	}

	if found {
		return manga, nil
	}

	manga, err = a.getByID(ctx, id)
	if err != nil {
		return nil, err
	}

	err = a.cacheSetId(id, manga)
	if err != nil {
		return nil, err
	}

	return manga, nil
}

func (a *Anilist) getByID(
	ctx context.Context,
	id int,
) (*AnilistManga, error) {
	a.options.Log(fmt.Sprintf("Searching manga with id %d on Anilist", id))

	body := &anilistRequestBody{
		Query: anilistQuerySearchByID,
		Variables: map[string]any{
			"id": id,
		},
	}

	data, err := sendRequest[struct {
		Media *AnilistManga `json:"media"`
	}](ctx, a, body)

	if err != nil {
		return nil, err
	}

	manga := data.Media
	if manga == nil {
		return nil, errors.New("manga by id not found")
	}

	return manga, nil
}

func (a *Anilist) SearchMangas(
	ctx context.Context,
	query string,
) ([]*AnilistManga, error) {
	query = unifyString(query)

	a.options.Log("Searching manga on Anilist...")

	{
		found, ids, err := a.cacheStatusQuery(query)
		if err != nil {
			return nil, err
		}

		if found {
			var mangas []*AnilistManga

			for _, id := range ids {
				manga, err := a.GetByID(ctx, id)
				if err != nil {
					return nil, err
				}

				mangas = append(mangas, manga)
			}

			return mangas, nil
		}
	}

	mangas, err := a.searchMangas(ctx, query)
	if err != nil {
		return nil, err
	}

	var ids = make([]int, len(mangas))
	for i, manga := range mangas {
		err := a.cacheSetId(manga.ID, manga)
		if err != nil {
			return nil, err
		}

		ids[i] = manga.ID
	}

	err = a.cacheSetQuery(query, ids)
	if err != nil {
		return nil, err
	}

	return mangas, nil
}

func (a *Anilist) searchMangas(
	ctx context.Context,
	query string,
) ([]*AnilistManga, error) {
	body := &anilistRequestBody{
		Query: anilistQuerySearchByName,
		Variables: map[string]any{
			"query": query,
		},
	}

	data, err := sendRequest[struct {
		Page struct {
			Media []*AnilistManga `json:"media"`
		} `json:"page"`
	}](ctx, a, body)

	if err != nil {
		return nil, err
	}

	mangas := data.Page.Media

	a.options.Log(fmt.Sprintf("Found %d manga(s) on Anilist.", len(mangas)))

	return mangas, nil
}

type anilistResponse[Data any] struct {
	Errors []struct {
		Message string `json:"message"`
		Status  int    `json:"status"`
	} `json:"errors"`
	Data Data `json:"data"`
}

func sendRequest[Data any](
	ctx context.Context,
	anilist *Anilist,
	requestBody *anilistRequestBody,
) (data Data, err error) {
	marshalled, err := json.Marshal(requestBody)
	if err != nil {
		return data, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, anilistUrlApi, bytes.NewReader(marshalled))
	if err != nil {
		return data, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	response, err := anilist.options.HTTPClient.Do(request)
	if err != nil {
		return data, err
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
			return data, err
		}

		anilist.options.Log(fmt.Sprintf("Rate limited. Retrying in %d seconds...", seconds))

		select {
		case <-time.After(time.Duration(seconds) * time.Second):
		case <-ctx.Done():
			return data, ctx.Err()
		}

		return sendRequest[Data](ctx, anilist, requestBody)
	}

	if response.StatusCode != http.StatusOK {
		return data, fmt.Errorf(response.Status)
	}

	var body anilistResponse[Data]

	err = json.NewDecoder(response.Body).Decode(&body)
	if err != nil {
		return data, err
	}

	if body.Errors != nil {
		err := body.Errors[0]
		return data, errors.New(err.Message)
	}

	return body.Data, nil
}

func (a *Anilist) FindClosestManga(
	ctx context.Context,
	title string,
) (*AnilistManga, bool, error) {
	a.options.Log("Finding closest manga on Anilist...")

	title = unifyString(title)

	found, id, err := a.cacheStatusTitle(title)
	if err != nil {
		return nil, false, err
	}

	if found {
		found, manga, err := a.cacheStatusId(id)
		if err != nil {
			return nil, false, err
		}

		if found {
			return manga, true, nil
		}
	}

	manga, ok, err := a.findClosestManga(
		ctx,
		title,
		title,
		3,
		0,
		3,
	)
	if err != nil {
		return nil, false, err
	}

	err = a.cacheSetTitle(title, manga.ID)
	if err != nil {
		return nil, false, err
	}

	if !ok {
		return nil, false, nil
	}

	return manga, true, nil
}

func (a *Anilist) findClosestManga(
	ctx context.Context,
	originalTitle, currentTitle string,
	step, try, limit int,
) (*AnilistManga, bool, error) {
	if try >= limit {
		return nil, false, nil
	}

	a.options.Log(
		fmt.Sprintf("Finding closest manga on Anilist (try %d/%d)", try+1, limit),
	)

	mangas, err := a.SearchMangas(ctx, currentTitle)
	if err != nil {
		return nil, false, err
	}

	if len(mangas) == 0 {
		// try again with a different title
		// remove `step` characters from the end of the title
		// avoid removing the last character or going out of bounds
		var newLen int
		if len(currentTitle) > step {
			newLen = len(currentTitle) - step
		} else if len(currentTitle) > 1 {
			newLen = len(currentTitle) - 1
		} else {
			// trigger limit, proceeding further will only make things worse
			return nil, false, nil
		}

		currentTitle = currentTitle[:newLen]
		return a.findClosestManga(ctx, originalTitle, currentTitle, step, try+1, limit)
	}

	// find the closest match
	closest, ok := a.options.GetClosestManga(currentTitle, mangas)

	if !ok {
		return nil, false, nil
	}

	a.options.Log(fmt.Sprintf("Found closest manga on Anilist: %q #%d", closest.String(), closest.ID))
	return closest, true, nil
}

func (a *Anilist) BindTitleWithID(title string, anilistMangaId int) error {
	return a.options.TitleToIDStore.Set(title, anilistMangaId)
}

func (a *Anilist) MakeMangaWithAnilist(
	ctx context.Context,
	manga MangaInfo,
) (*MangaWithAnilist, error) {
	anilistManga, ok, err := a.FindClosestManga(ctx, manga.Title)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, errors.New("anilist manga not found")
	}

	return &MangaWithAnilist{
		Info:    manga,
		Anilist: anilistManga,
	}, nil
}

func (a *Anilist) MakeChapterWithAnilist(ctx context.Context, chapter ChapterInfo) (*ChapterOfMangaWithAnilist, error) {
	mangaWithAnilist, err := a.MakeMangaWithAnilist(ctx, chapter.VolumeInfo().MangaInfo())
	if err != nil {
		return nil, err
	}

	return &ChapterOfMangaWithAnilist{
		Info:             chapter,
		MangaWithAnilist: mangaWithAnilist,
	}, nil
}
