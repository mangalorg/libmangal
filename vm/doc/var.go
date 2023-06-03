package luadoc

import lua "github.com/yuin/gopher-lua"

type Var struct {
	Name        string
	Description string
	Value       lua.LValue
	Type        string
}
