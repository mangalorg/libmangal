package crypto

import (
	luadoc "github.com/metafates/libmangal/vm/doc"
	"github.com/metafates/libmangal/vm/lib/crypto/aes"
	"github.com/metafates/libmangal/vm/lib/crypto/md5"
	"github.com/metafates/libmangal/vm/lib/crypto/sha1"
	"github.com/metafates/libmangal/vm/lib/crypto/sha256"
	"github.com/metafates/libmangal/vm/lib/crypto/sha512"
	lua "github.com/yuin/gopher-lua"
)

func Lib(L *lua.LState) *luadoc.Lib {
	return &luadoc.Lib{
		Name:        "crypto",
		Description: "Various cryptographic functions.",
		Libs: []*luadoc.Lib{
			aes.Lib(),
			md5.Lib(),
			sha1.Lib(),
			sha256.Lib(),
			sha512.Lib(),
		},
	}
}
