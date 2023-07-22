package libmangal

import "fmt"

type (
	MetadataError struct {
		error
	}

	AnilistError struct {
		error
	}
)

func (a AnilistError) Error() string {
	return fmt.Sprintf("anilist error: %s", a.error)
}
