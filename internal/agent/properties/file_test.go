package properties

import (
	"maps"
	"testing"
)

// As written by Minecraft, with a few hand edits.
const serverProperties = `#Minecraft server properties
#Thu Oct 01 12:00:00 UTC 2026
level-type=minecraft\:normal
motd=\u00A7aWelcome to the \u00A76Lobby
max-players=20
spaced\ key = value with spaces
long=first \
    second
! old style comment
pvp=true
`

func TestParse(t *testing.T) {
	got := map[string]string{}
	for _, e := range parse(serverProperties) {
		if e.key != "" {
			got[e.key] = e.value
		}
	}
	want := map[string]string{
		"level-type":  "minecraft:normal",
		"motd":        "§aWelcome to the §6Lobby",
		"max-players": "20",
		"spaced key":  "value with spaces",
		"long":        "first second",
		"pvp":         "true",
	}
	if !maps.Equal(got, want) {
		t.Fatalf("parse() = %q, want %q", got, want)
	}
}

func TestUpdate(t *testing.T) {
	changes := map[string]string{"motd": " Hello\nWörld 😀 #1", "max-players": "50", "white-list": "true"}
	out := update(parse(serverProperties), changes)
	want := `#Minecraft server properties
#Thu Oct 01 12:00:00 UTC 2026
level-type=minecraft\:normal
motd=\ Hello\nW\u00F6rld \uD83D\uDE00 \#1
max-players=50
spaced\ key = value with spaces
long=first \
    second
! old style comment
pvp=true
white-list=true
`
	if out != want {
		t.Fatalf("update() =\n%s\nwant\n%s", out, want)
	}
	// What was written reads back the same.
	for _, e := range parse(out) {
		if v, ok := changes[e.key]; ok && e.value != v {
			t.Errorf("%s = %q, want %q", e.key, e.value, v)
		}
	}
}
