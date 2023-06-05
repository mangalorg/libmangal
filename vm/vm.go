package vm

import (
	"github.com/mangalorg/libmangal/vm/lib"
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

func NewState(options *Options) *lua.LState {
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

	lib.Preload(state, &lib.Options{
		HTTPClient: options.HTTPClient,
		FS:         options.FS,
		HTTPStore:  options.HTTPStore,
	})

	return state
}
