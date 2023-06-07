package libmangal

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/yuin/gopher-lua"
	"net/url"
)

func errManga(err error) error {
	return errors.Wrap(err, "manga")
}

type Manga struct {
	Title    string
	Url      string
	Id       string
	CoverUrl string

	table *lua.LTable
}

func (m *Manga) String() string {
	return m.Title
}

func (m *Manga) validate() error {
	if m.Title == "" {
		return errManga(fmt.Errorf("title must be non-empty"))
	}

	if m.Id == "" {
		return errManga(fmt.Errorf("id must be non-empty"))
	}

	if m.Url != "" {
		if _, err := url.Parse(m.Url); err != nil {
			return errManga(err)
		}
	}

	if m.CoverUrl != "" {
		if _, err := url.Parse(m.CoverUrl); err != nil {
			return errManga(err)
		}
	}

	return nil
}

func (m *Manga) NameData() MangaNameData {
	return MangaNameData{
		Title: m.Title,
		Id:    m.Id,
	}
}

type MangaNameData struct {
	Title string
	Id    string
}
