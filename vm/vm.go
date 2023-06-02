package vm

import (
	"github.com/metafates/libmangal/vm/lib"
	"github.com/spf13/afero"
	lua "github.com/yuin/gopher-lua"
	"net/http"
)

type Options struct {
	HTTPClient *http.Client
	FS         afero.Fs
}

func NewState(options Options) *lua.LState {
	if options.FS == nil {
		panic("FS is nil")
	}

	if options.HTTPClient == nil {
		panic("HTTPClient is nil")
	}

	libs := []lua.LGFunction{
		lua.OpenBase,
		lua.OpenTable,
		lua.OpenString,
		lua.OpenMath,
		lua.OpenPackage,
		lua.OpenIo,
		lua.OpenCoroutine,
		lua.OpenChannel,
	}

	state := lua.NewState(lua.Options{
		SkipOpenLibs: true,
	})

	for _, injectLib := range libs {
		injectLib(state)
	}

	lib.Preload(state, lib.Options{
		HTTPClient: options.HTTPClient,
		FS:         options.FS,
	})

	return state
}
