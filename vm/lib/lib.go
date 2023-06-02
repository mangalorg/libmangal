package lib

import (
	"github.com/cjoudrey/gluahttp"
	"github.com/samber/lo"
	"github.com/spf13/afero"
	lua "github.com/yuin/gopher-lua"
	"net/http"
)

type Options struct {
	HTTPClient *http.Client
	FS         afero.Fs
}

func Preload(state *lua.LState, options Options) {
	for _, t := range []lo.Tuple2[string, lua.LGFunction]{
		{"http", gluahttp.NewHttpModule(options.HTTPClient).Loader},
	} {
		state.PreloadModule(t.A, t.B)
	}
}
