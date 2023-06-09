package libmangal

import "strconv"

func (a *Anilist) cacheStatusQuery(
	query string,
) (found bool, ids []int, err error) {
	found, err = a.options.QueryToIDsStore.Get(query, &ids)
	return
}

func (a *Anilist) cacheSetQuery(
	query string,
	ids []int,
) error {
	return a.options.QueryToIDsStore.Set(query, ids)
}

func (a *Anilist) cacheStatusTitle(
	title string,
) (found bool, id int, err error) {
	found, err = a.options.TitleToIDStore.Get(title, &id)
	return
}

func (a *Anilist) cacheSetTitle(
	title string,
	id int,
) error {
	return a.options.TitleToIDStore.Set(title, id)
}

func (a *Anilist) cacheStatusId(
	id int,
) (found bool, manga AnilistManga, err error) {
	found, err = a.options.IDToMangaStore.Get(strconv.Itoa(id), &manga)
	return
}

func (a *Anilist) cacheSetId(
	id int,
	manga AnilistManga,
) error {
	return a.options.IDToMangaStore.Set(strconv.Itoa(id), manga)
}
