package libmangal

import (
	"fmt"
	"github.com/pkg/errors"
	lua "github.com/yuin/gopher-lua"
)

func errChapter(err error) error {
	return errors.Wrap(err, "chapter")
}

type Chapter struct {
	Title  string
	Url    string
	Number string

	table *lua.LTable
	manga *Manga
}

func (c *Chapter) validate() error {
	if c.Title == "" {
		return errChapter(fmt.Errorf("title must be non-empty"))
	}

	return nil
}

func (c *Chapter) NameData() ChapterNameData {
	return ChapterNameData{
		Title:      c.Title,
		Number:     c.Number,
		MangaTitle: c.manga.Title,
	}
}

type ChapterNameData struct {
	Title      string
	Number     string
	MangaTitle string
}
