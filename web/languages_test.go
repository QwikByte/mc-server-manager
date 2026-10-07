package web

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The master accepts the languages the panel ships, as the names of their files.
func TestLanguages(t *testing.T) {
	entries, err := os.ReadDir("src/locales")
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		files = append(files, strings.TrimSuffix(e.Name(), ".json"))
	}
	if !slices.Equal(slices.Sorted(slices.Values(files)), slices.Sorted(slices.Values(Languages))) {
		t.Errorf("Languages = %v, but src/locales has %v", Languages, files)
	}
	// A language, with a script or region if it needs one, e.g. de, pt-BR or zh-Hant.
	tag := regexp.MustCompile(`^[a-z]{2,3}(-[A-Z][a-z]{3})?(-([A-Z]{2}|[0-9]{3}))?$`)
	for _, l := range Languages {
		if !tag.MatchString(l) {
			t.Errorf("%q isn't a language tag", l)
		}
	}
}
