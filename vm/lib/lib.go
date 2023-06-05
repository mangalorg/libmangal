package lib

import (
	luadoc "github.com/mangalorg/libmangal/vm/doc"
	"github.com/mangalorg/libmangal/vm/lib/crypto"
	"github.com/mangalorg/libmangal/vm/lib/encoding"
	"github.com/mangalorg/libmangal/vm/lib/html"
	httpLib "github.com/mangalorg/libmangal/vm/lib/http"
	"github.com/mangalorg/libmangal/vm/lib/js"
	"github.com/mangalorg/libmangal/vm/lib/levenshtein"
	"github.com/mangalorg/libmangal/vm/lib/regexp"
	"github.com/mangalorg/libmangal/vm/lib/strings"
	"github.com/mangalorg/libmangal/vm/lib/time"
	"github.com/mangalorg/libmangal/vm/lib/urls"
	"github.com/mangalorg/libmangal/vm/lib/util"
	"github.com/philippgille/gokv"
	"github.com/philippgille/gokv/syncmap"
	"github.com/spf13/afero"
	lua "github.com/yuin/gopher-lua"
	"net/http"
)

type Options struct {
	HTTPClient *http.Client
	HTTPStore  gokv.Store
	FS         afero.Fs
}

func DefaultOptions() *Options {
	return &Options{
		HTTPClient: &http.Client{},
		HTTPStore:  syncmap.NewStore(syncmap.DefaultOptions),
		FS:         afero.NewMemMapFs(),
	}
}

func Lib(L *lua.LState, options *Options) *luadoc.Lib {
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
			httpLib.Lib(httpLib.LibOptions{
				HTTPClient: options.HTTPClient,
				HTTPStore:  options.HTTPStore,
			}),
		},
	}
}

func Preload(L *lua.LState, options *Options) {
	lib := Lib(L, options)
	L.PreloadModule(lib.Name, lib.Loader())
}
