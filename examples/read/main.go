package main

import (
	"bytes"
	"context"
	_ "embed"
	"github.com/mangalorg/libmangal"
)

//go:embed provider.lua
var script []byte

const query = "chainsaw man"

func main() {
	client := libmangal.NewClient(libmangal.DefaultClientOptions())
	providerHandle, err := client.ProviderHandleFromReader(bytes.NewReader(script))
	if err != nil {
		panic(err)
	}

	provider, err := providerHandle.LoadProvider(
		context.Background(),
		libmangal.DefaultProviderLoadOptions(),
	)
	if err != nil {
		panic(err)
	}

	mangas, err := provider.SearchMangas(
		context.Background(),
		query,
	)
	if err != nil {
		panic(err)
	}

	chapters, err := provider.MangaChapters(
		context.Background(),
		mangas[0],
	)
	if err != nil {
		panic(err)
	}

	err = provider.ReadChapter(
		context.Background(),
		chapters[0],
		libmangal.DefaultReadOptions(),
	)
	if err != nil {
		panic(err)
	}
}
