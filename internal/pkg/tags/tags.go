package tag

import (
	"strings"
)

// Parse splits a struct field's json tag into its name and
// comma-separated options.
// from here - https://github.com/golang/go/blob/master/src/encoding/json/tags.go#L17
func Parse(tag string) string {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx]
	}
	return tag
}
