package libmangal

import lua "github.com/yuin/gopher-lua"

type Chapter struct {
	Title string

	table *lua.LTable
}
