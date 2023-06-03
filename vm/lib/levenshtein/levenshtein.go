package levenshtein

import (
	levenshtein "github.com/ka-weihe/fast-levenshtein"
	luadoc "github.com/mangalorg/libmangal/vm/doc"
	lua "github.com/yuin/gopher-lua"
)

func Lib() *luadoc.Lib {
	return &luadoc.Lib{
		Name:        "levenshtein",
		Description: "Levenshtein distance algorithm",
		Funcs: []*luadoc.Func{
			{
				Name:        "distance",
				Description: "Compute Levenshtein distance between two strings",
				Value:       distance,
				Params: []*luadoc.Param{
					{
						Name: "s1",
						Type: luadoc.String,
					},
					{
						Name: "s2",
						Type: luadoc.String,
					},
				},
				Returns: []*luadoc.Param{
					{
						Name:        "distance",
						Description: "Levenshtein distance between s1 and s2",
						Type:        luadoc.Number,
					},
				},
			},
		},
	}
}

func distance(L *lua.LState) int {
	s1 := L.CheckString(1)
	s2 := L.CheckString(2)

	L.Push(lua.LNumber(levenshtein.Distance(s1, s2)))

	return 1
}
