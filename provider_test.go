package libmangal

import (
	"context"
	. "github.com/smartystreets/goconvey/convey"
	"testing"
)

var defaultClient = NewClient(Options{
	ProvidersPaths: []string{"./testdata/provider.lua"},
})

func TestClient_Providers(t *testing.T) {
	Convey("When ProvidersHandles() is called", t, func() {
		providers := defaultClient.ProvidersHandles()

		Convey("Then it should return a list of providers", func() {
			So(len(providers), ShouldEqual, 1)

			Convey("And the first provider should have the correct filename", func() {
				So(providers[0].Filename(), ShouldEqual, "provider.lua")
			})
		})
	})
}

func TestProviderHandle_Get(t *testing.T) {
	Convey("Given a provider handle", t, func() {
		handle := defaultClient.ProvidersHandles()[0]

		Convey("When LoadProvider() is called", func() {
			provider, err := handle.LoadProvider()

			Convey("Then it should return a provider", func() {
				So(err, ShouldBeNil)

				Convey("And the provider should have the correct info", func() {
					info := provider.Info()
					So(info.Name, ShouldEqual, "Name")
					So(info.Description, ShouldEqual, "Description")
					So(info.Version, ShouldEqual, "0.1.0")
				})
			})
		})
	})
}

func TestProvider_SearchMangas(t *testing.T) {
	Convey("Given a provider", t, func() {
		handle := defaultClient.ProvidersHandles()[0]
		provider, err := handle.LoadProvider()
		So(err, ShouldBeNil)

		Convey("When SearchMangas() is called", func() {
			const query = "naruto"
			mangas, err := provider.SearchMangas(context.Background(), query)

			Convey("Then it should return a list of mangas", func() {
				So(err, ShouldBeNil)
				So(len(mangas), ShouldEqual, 1)

				Convey("And the first manga should have the correct info", func() {
					manga := mangas[0]
					So(manga.Title, ShouldEqual, query)
				})
			})
		})
	})
}

func TestProvider_MangaChapters(t *testing.T) {
	Convey("Given a manga", t, func() {
		handle := defaultClient.ProvidersHandles()[0]
		provider, err := handle.LoadProvider()
		So(err, ShouldBeNil)

		mangas, err := provider.SearchMangas(context.Background(), "naruto")
		So(err, ShouldBeNil)
		So(len(mangas), ShouldEqual, 1)

		manga := mangas[0]

		Convey("When MangaChapters() is called", func() {
			chapters, err := provider.MangaChapters(context.Background(), manga)

			Convey("Then it should return a list of chapters", func() {
				So(err, ShouldBeNil)
				So(len(chapters), ShouldEqual, 1)

				Convey("And the first chapter should have the correct info", func() {
					chapter := chapters[0]
					So(chapter.Title, ShouldEqual, "Chapter 1")
				})
			})
		})
	})
}

func TestProvider_ChapterPages(t *testing.T) {
	Convey("Given a chapter", t, func() {
		handle := defaultClient.ProvidersHandles()[0]
		provider, err := handle.LoadProvider()
		So(err, ShouldBeNil)

		mangas, err := provider.SearchMangas(context.Background(), "naruto")
		So(err, ShouldBeNil)
		So(len(mangas), ShouldEqual, 1)

		manga := mangas[0]

		chapters, err := provider.MangaChapters(context.Background(), manga)
		So(err, ShouldBeNil)
		So(len(chapters), ShouldEqual, 1)

		chapter := chapters[0]

		Convey("When ChapterPages() is called", func() {
			pages, err := provider.ChapterPages(context.Background(), chapter)

			Convey("Then it should return a list of pages", func() {
				So(err, ShouldBeNil)
				So(len(pages), ShouldEqual, 1)

				Convey("And the first page should have the correct info", func() {
					page := pages[0]
					So(page.Url, ShouldEqual, "https://example.com/image.jpg")
					So(page.Headers, ShouldResemble, map[string]string{
						"Referer":    "https://example.com",
						"User-Agent": "libmangal",
					})
				})
			})
		})
	})
}
