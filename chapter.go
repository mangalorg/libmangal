package libmangal

import lua "github.com/yuin/gopher-lua"

type Chapter struct {
	Title  string
	Url    string
	Number string

	table *lua.LTable
	manga *Manga
}

func (c *Chapter) nameData() ChapterNameData {
	return ChapterNameData{
		Title:      c.Title,
		Number:     c.Number,
		MangaTitle: c.manga.Title,
	}
}
