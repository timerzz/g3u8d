package utils

import (
	"path"
	"strings"
)

func ResolveURL(base, url string) string {
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		return url
	}
	return path.Join(base, url)
}
