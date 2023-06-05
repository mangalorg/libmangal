package aes

import (
	"crypto/aes"
	luadoc "github.com/mangalorg/libmangal/vm/doc"
	lua "github.com/yuin/gopher-lua"
)

func Lib() *luadoc.Lib {
	return &luadoc.Lib{
		Name:        "aes",
		Description: "AES encryption and decryption.",
		Funcs: []*luadoc.Func{
			{
				Name:        "encrypt",
				Description: "Encrypts a string using AES.",
				Value:       encrypt,
				Params: []*luadoc.Param{
					{
						Name:        "key",
						Description: "The key to use for encryption.",
						Type:        luadoc.String,
					},
					{
						Name:        "value",
						Description: "The string to encrypt.",
						Type:        luadoc.String,
					},
				},
				Returns: []*luadoc.Param{
					{
						Name:        "encrypted",
						Description: "The encrypted string.",
						Type:        luadoc.String,
					},
				},
			},
			{
				Name:        "decrypt",
				Description: "Decrypts a string using AES.",
				Value:       decrypt,
				Params: []*luadoc.Param{
					{
						Name:        "key",
						Description: "The key to use for decryption.",
						Type:        luadoc.String,
					},
					{
						Name:        "value",
						Description: "The string to decrypt.",
						Type:        luadoc.String,
					},
				},
				Returns: []*luadoc.Param{
					{
						Name:        "decrypted",
						Description: "The decrypted string.",
						Type:        luadoc.String,
					},
				},
			},
		},
	}
}

func encrypt(L *lua.LState) int {
	key := L.CheckString(1)
	value := L.CheckString(2)

	cipher, err := aes.NewCipher([]byte(key))
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	encrypted := make([]byte, len(value))
	cipher.Encrypt(encrypted, []byte(value))

	L.Push(lua.LString(encrypted))
	return 1
}

func decrypt(L *lua.LState) int {
	key := L.CheckString(1)
	value := L.CheckString(2)

	cipher, err := aes.NewCipher([]byte(key))
	if err != nil {
		L.RaiseError(err.Error())
		return 0
	}

	decrypted := make([]byte, len(value))
	cipher.Decrypt(decrypted, []byte(value))

	L.Push(lua.LString(decrypted))
	return 1
}
