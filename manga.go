package libmangal

import lua "github.com/yuin/gopher-lua"

type Manga struct {
	Title string

	table *lua.LTable
}
