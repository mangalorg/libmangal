package encoding

import (
	luadoc "github.com/metafates/libmangal/vm/doc"
	"github.com/metafates/libmangal/vm/lib/encoding/base64"
	"github.com/metafates/libmangal/vm/lib/encoding/json"
	lua "github.com/yuin/gopher-lua"
)

func Lib(L *lua.LState) *luadoc.Lib {
	return &luadoc.Lib{
		Name:        "encoding",
		Description: "",
		Libs: []*luadoc.Lib{
			base64.Lib(L),
			json.Lib(),
		},
	}
}
