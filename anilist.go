package libmangal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const anilistAPIURL = "https://graphql.anilist.co"

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
	accessToken string
	options     AnilistOptions
	logger      *Logger
}

// NewAnilist constructs new Anilist client
func NewAnilist(options AnilistOptions) Anilist {
	var accessToken string
	found, err := options.AccessTokenStore.Get(anilistStoreAccessCodeStoreKey, &accessToken)

	anilist := Anilist{
		options: options,
		logger:  options.Logger,
	}

	if err == nil && found {
		anilist.accessToken = accessToken
	}

	return anilist
}

// GetByID gets anilist manga by its id
func (a *Anilist) GetByID(
	ctx context.Context,
	id int,
) (AnilistManga, bool, error) {
	found, manga, err := a.cacheStatusId(id)
	if err != nil {
		return AnilistManga{}, false, AnilistError{err}
	}

	if found {
		return manga, true, nil
	}

	manga, ok, err := a.getByID(ctx, id)
	if err != nil {
		return AnilistManga{}, false, AnilistError{err}
	}

	if !ok {
		return AnilistManga{}, false, nil
	}

	err = a.cacheSetId(id, manga)
	if err != nil {
		return AnilistManga{}, false, AnilistError{err}
	}

	return manga, true, nil
}

func (a *Anilist) getByID(
	ctx context.Context,
	id int,
) (AnilistManga, bool, error) {
	a.logger.Log(fmt.Sprintf("Searching manga with id %d on AnilistSearch", id))

	body := anilistRequestBody{
		Query: anilistQuerySearchByID,
		Variables: map[string]any{
			"id": id,
		},
	}

	data, err := sendRequest[struct {
		Media *AnilistManga `json:"media"`
	}](ctx, a, body)

	if err != nil {
		return AnilistManga{}, false, err
	}

	manga := data.Media
	if manga == nil {
		return AnilistManga{}, false, nil
	}

	return *manga, true, nil
}

func (a *Anilist) SearchMangas(
	ctx context.Context,
	query string,
) ([]AnilistManga, error) {
	a.logger.Log("Searching manga on AnilistSearch...")

	{
		found, ids, err := a.cacheStatusQuery(query)
		if err != nil {
			return nil, AnilistError{err}
		}

		if found {
			var mangas []AnilistManga

			for _, id := range ids {
				manga, ok, err := a.GetByID(ctx, id)
				if err != nil {
					return nil, err
				}

				if ok {
					mangas = append(mangas, manga)
				}
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
			return nil, AnilistError{err}
		}

		ids[i] = manga.ID
	}

	err = a.cacheSetQuery(query, ids)
	if err != nil {
		return nil, AnilistError{err}
	}

	return mangas, nil
}

func (a *Anilist) searchMangas(
	ctx context.Context,
	query string,
) ([]AnilistManga, error) {
	body := anilistRequestBody{
		Query: anilistQuerySearchByName,
		Variables: map[string]any{
			"query": query,
		},
	}

	data, err := sendRequest[struct {
		Page struct {
			Media []AnilistManga `json:"media"`
		} `json:"page"`
	}](ctx, a, body)

	if err != nil {
		return nil, err
	}

	mangas := data.Page.Media

	a.logger.Log(fmt.Sprintf("Found %d manga(s) on AnilistSearch.", len(mangas)))

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
	requestBody anilistRequestBody,
) (data Data, err error) {
	marshalled, err := json.Marshal(requestBody)
	if err != nil {
		return data, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, anilistAPIURL, bytes.NewReader(marshalled))
	if err != nil {
		return data, err
	}

	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	if anilist.IsAuthorized() {
		request.Header.Set(
			"Authorization",
			fmt.Sprintf("Bearer %s", anilist.accessToken),
		)
	}

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

		anilist.logger.Log(fmt.Sprintf("Rate limited. Retrying in %d seconds...", seconds))

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
) (AnilistManga, bool, error) {
	a.logger.Log("Finding closest manga on AnilistSearch...")

	found, id, err := a.cacheStatusTitle(title)
	if err != nil {
		return AnilistManga{}, false, AnilistError{err}
	}

	if found {
		found, manga, err := a.cacheStatusId(id)
		if err != nil {
			return AnilistManga{}, false, AnilistError{err}
		}

		if found {
			return manga, true, nil
		}
	}

	manga, ok, err := a.findClosestManga(
		ctx,
		title,
		3,
		3,
	)
	if err != nil {
		return AnilistManga{}, false, AnilistError{err}
	}

	if !ok {
		return AnilistManga{}, false, nil
	}

	err = a.cacheSetTitle(title, manga.ID)
	if err != nil {
		return AnilistManga{}, false, AnilistError{err}
	}

	return manga, true, nil
}

func (a *Anilist) findClosestManga(
	ctx context.Context,
	title string,
	step,
	tries int,
) (AnilistManga, bool, error) {
	for i := 0; i < tries; i++ {
		a.logger.Log(
			fmt.Sprintf("Finding closest manga on AnilistSearch (try %d/%d)", i+1, tries),
		)

		mangas, err := a.SearchMangas(ctx, title)
		if err != nil {
			return AnilistManga{}, false, err
		}

		if len(mangas) > 0 {
			closest := mangas[0]
			a.logger.Log(fmt.Sprintf("Found closest manga on AnilistSearch: %q #%d", closest.String(), closest.ID))
			return closest, true, nil
		}

		// try again with a different title
		// remove `step` characters from the end of the title
		// avoid removing the last character or going out of bounds
		var newLen int

		title = strings.TrimSpace(title)

		if len(title) > step {
			newLen = len(title) - step
		} else if len(title) > 1 {
			newLen = len(title) - 1
		} else {
			break
		}

		title = title[:newLen]
	}

	return AnilistManga{}, false, nil
}

func (a *Anilist) BindTitleWithID(title string, anilistMangaId int) error {
	err := a.options.TitleToIDStore.Set(title, anilistMangaId)
	if err != nil {
		return AnilistError{err}
	}

	return nil
}

func (a *Anilist) SetMangaProgress(ctx context.Context, mangaID, chapterNumber int) error {
	if !a.IsAuthorized() {
		return AnilistError{errors.New("not authorized")}
	}

	_, err := sendRequest[struct {
		SaveMediaListEntry struct {
			ID int `json:"id"`
		} `json:"SaveMediaListEntry"`
	}](
		ctx,
		a,
		anilistRequestBody{
			Query: anilistMutationSaveProgress,
			Variables: map[string]any{
				"id":       mangaID,
				"progress": chapterNumber,
			},
		},
	)

	if err != nil {
		return AnilistError{err}
	}

	return nil
}

func (a *Anilist) MakeMangaWithAnilist(
	ctx context.Context,
	manga Manga,
) (MangaWithAnilist, bool, error) {
	var title string
	info := manga.Info()

	if info.AnilistSearch != "" {
		title = info.AnilistSearch
	} else {
		title = info.Title
	}

	anilistManga, ok, err := a.FindClosestManga(ctx, title)
	if err != nil {
		return MangaWithAnilist{}, false, AnilistError{err}
	}

	if !ok {
		return MangaWithAnilist{}, false, nil
	}

	return MangaWithAnilist{
		Manga:   manga,
		Anilist: anilistManga,
	}, true, nil
}

func (a *Anilist) MakeChapterWithAnilist(
	ctx context.Context,
	chapter Chapter,
) (ChapterOfMangaWithAnilist, bool, error) {
	mangaWithAnilist, ok, err := a.MakeMangaWithAnilist(ctx, chapter.Volume().Manga())
	if err != nil {
		return ChapterOfMangaWithAnilist{}, false, AnilistError{err}
	}

	if !ok {
		return ChapterOfMangaWithAnilist{}, false, nil
	}

	return ChapterOfMangaWithAnilist{
		Chapter:          chapter,
		MangaWithAnilist: mangaWithAnilist,
	}, true, nil
}
