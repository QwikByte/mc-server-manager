package plugin

import (
	"testing"

	"github.com/QwikByte/noryx/internal/master/httpapi"
	"github.com/QwikByte/noryx/internal/master/modrinth"
)

func TestPick(t *testing.T) {
	version := func(id, channel string) modrinth.Version {
		return modrinth.Version{ID: id, VersionType: channel, Files: []modrinth.File{{Filename: id + ".jar"}}}
	}
	release, beta, alpha := version("release", "release"), version("beta", "beta"), version("alpha", "alpha")
	paper := target{loaders: []string{"paper"}, gameVersion: "26.2"}
	for _, tc := range []struct {
		name     string
		versions []modrinth.Version // the newest first
		chosen   string
		want     string // the ID of the version, or the error
		channel  string
	}{
		{name: "newest release", versions: []modrinth.Version{beta, release}, want: "release"},
		{name: "newest pre-release without a release", versions: []modrinth.Version{alpha, beta}, want: "alpha", channel: "alpha"},
		{name: "chosen pre-release", versions: []modrinth.Version{beta, release}, chosen: "beta", want: "beta", channel: "beta"},
		{name: "chosen version for another server", versions: []modrinth.Version{release}, chosen: "other", want: "The chosen version of ViaVersion doesn't run on Paper 26.2."},
		{name: "no version", want: "ViaVersion has no version for Paper 26.2."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &installation{chosen: map[string]string{"via": tc.chosen}}
			// Looked up once per installation; set here, the catalogue isn't asked.
			_, _ = r.versions.get("via|paper|26.2", func() ([]modrinth.Version, error) { return tc.versions, nil })
			_, _ = r.titles.get("via", func() (string, error) { return "ViaVersion", nil })
			v, err := r.pick(t.Context(), "via", paper)
			got := v.ID
			if err != nil {
				got = httpapi.Message(err)
			}
			if got != tc.want || preRelease(v) != tc.channel {
				t.Fatalf("pick = %q (channel %q), want %q (channel %q)", got, preRelease(v), tc.want, tc.channel)
			}
		})
	}
}
