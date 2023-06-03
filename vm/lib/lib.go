package lib

import (
	"github.com/cjoudrey/gluahttp"
	luadoc "github.com/mangalorg/libmangal/vm/doc"
	"github.com/mangalorg/libmangal/vm/lib/crypto"
	"github.com/mangalorg/libmangal/vm/lib/encoding"
	"github.com/mangalorg/libmangal/vm/lib/html"
	"github.com/mangalorg/libmangal/vm/lib/js"
	"github.com/mangalorg/libmangal/vm/lib/levenshtein"
	"github.com/mangalorg/libmangal/vm/lib/regexp"
	"github.com/mangalorg/libmangal/vm/lib/strings"
	"github.com/mangalorg/libmangal/vm/lib/time"
	"github.com/mangalorg/libmangal/vm/lib/urls"
	"github.com/mangalorg/libmangal/vm/lib/util"
	"github.com/samber/lo"
	"github.com/spf13/afero"
	lua "github.com/yuin/gopher-lua"
	"net/http"
)

func Lib(L *lua.LState) *luadoc.Lib {
	return &luadoc.Lib{
		Name:        "libmangal",
		Description: `libmangal lua SDK. Contains various utilities for making HTTP requests, working with JSON, HTML, and more.`,
		Libs: []*luadoc.Lib{
			regexp.Lib(L),
			strings.Lib(),
			crypto.Lib(L),
			js.Lib(),
			html.Lib(),
			levenshtein.Lib(),
			util.Lib(),
			time.Lib(),
			urls.Lib(),
			encoding.Lib(L),
			// TODO: add http lib
		},
	}
}

type Options struct {
	HTTPClient *http.Client
	FS         afero.Fs
}

func Preload(state *lua.LState, options Options) {
	for _, t := range []lo.Tuple2[string, lua.LGFunction]{
		{"http", gluahttp.NewHttpModule(options.HTTPClient).Loader},
		{"libmangal", Lib(state).Loader()},
	} {
		state.PreloadModule(t.A, t.B)
	}
}
