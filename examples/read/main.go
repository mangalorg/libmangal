package main

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"github.com/mangalorg/libmangal"
	"github.com/mangalorg/luaprovider"
	"log"
)

//go:embed mangapill.lua
var mangapill []byte

const query = "Chainsaw man"

func main() {
	loader, err := luaprovider.NewLoader(
		bytes.NewReader(mangapill),
		luaprovider.DefaultOptions(),
	)

	if err != nil {
		log.Fatal(err)
	}

	// log progress to stdout
	clientOptions := libmangal.DefaultClientOptions()
	clientOptions.Log = func(msg string) {
		fmt.Println(msg)
	}

	client, err := libmangal.NewClient(
		context.Background(),
		loader,
		clientOptions,
	)
	if err != nil {
		log.Fatal(err)
	}

	mangas, err := client.SearchMangas(
		context.Background(),
		query,
	)
	if err != nil {
		log.Fatal(err)
	}

	// get first manga
	manga := mangas[0]

	chapters, err := client.MangaChapters(
		context.Background(),
		manga,
	)
	if err != nil {
		log.Fatal(err)
	}

	// let read the first chapter
	chapter := chapters[0]

	err = client.ReadChapter(
		context.Background(),
		chapter,
		libmangal.DefaultReadOptions(),
	)
	if err != nil {
		log.Fatal(err)
	}
}
