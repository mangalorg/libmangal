package libmangal

import lua "github.com/yuin/gopher-lua"

type Manga struct {
	Title string
	Url   string

	table *lua.LTable
}

func (m *Manga) nameData() MangaNameData {
	return MangaNameData{
		Title: m.Title,
	}
}
