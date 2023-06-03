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

func (o *Options) fillDefaults() {
	if o.HTTPClient == nil {
		o.HTTPClient = &http.Client{}
	}

	if o.FS == nil {
		o.FS = afero.NewMemMapFs()
	}
}

func NewState(options Options) *lua.LState {
	options.fillDefaults()

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
