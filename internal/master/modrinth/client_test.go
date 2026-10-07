package modrinth

import (
	"slices"
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func TestEveryServerTypeHasLoaders(t *testing.T) {
	for value, name := range noryxv1.ServerType_name {
		typ := noryxv1.ServerType(value)
		if len(Loaders(typ)) == 0 && typ != noryxv1.ServerType_SERVER_TYPE_UNSPECIFIED && typ != noryxv1.ServerType_SERVER_TYPE_VANILLA {
			t.Errorf("%s has no loaders", name)
		}
	}
	if !slices.Contains(AllLoaders("mods"), "quilt") || slices.Contains(AllLoaders("plugins"), "quilt") {
		t.Error("Quilt doesn't load mods")
	}
}

func TestProjectOf(t *testing.T) {
	c := New(DefaultAPI, DefaultCDN)
	for url, want := range map[string]string{
		DefaultCDN + "data/AANobbMI/versions/IZskON6d/sodium.jar": "AANobbMI",
		DefaultCDN + "data/AANobbMI/icon.png":                     "",
		DefaultCDN + "data/../versions/IZskON6d/sodium.jar":       "",
		"https://example.com/data/AANobbMI/versions/x/sodium.jar": "",
	} {
		if got := c.ProjectOf(url); got != want {
			t.Errorf("ProjectOf(%q) = %q, want %q", url, got, want)
		}
	}
}
