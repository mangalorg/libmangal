package libmangal

import (
	"regexp"
	"strings"
)

func sanitizePath(path string) string {
	for _, ch := range invalidPathChars {
		path = strings.ReplaceAll(path, string(ch), "_")
	}

	// replace two or more consecutive underscores with one underscore
	return regexp.MustCompile(`_+`).ReplaceAllString(path, "_")
}
