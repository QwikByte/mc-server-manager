package network

import (
	"slices"
	"strings"
	"testing"
)

func TestMessageCommands(t *testing.T) {
	for _, tc := range []struct {
		m      Message
		target string
		want   []string
	}{
		{Message{Text: " Restart soon "}, Everyone, []string{"say Restart soon"}},
		{Message{Kind: "chat", Text: `Hi "there" \ {}`}, "Alex", []string{
			`minecraft:tellraw Alex {"translate":"commands.message.display.incoming","with":["Server","Hi \"there\" \\ {}"],"color":"gray","italic":true}`,
		}},
		{Message{Kind: "title", Text: "Welcome", Subtitle: "to <the> network"}, Everyone, []string{
			"minecraft:title @a subtitle {\"text\":\"to \\u003cthe\\u003e network\"}", `minecraft:title @a title {"text":"Welcome"}`,
		}},
		{Message{Kind: "actionbar", Text: "Hi"}, ".Bedrock_Kid", []string{`minecraft:title .Bedrock_Kid actionbar {"text":"Hi"}`}},
	} {
		got, err := tc.m.Commands(tc.target)
		if err != nil || !slices.Equal(got, tc.want) {
			t.Errorf("%+v to %s = %q, %v, want %q", tc.m, tc.target, got, err, tc.want)
		}
	}
	for _, tc := range []struct {
		m      Message
		target string
	}{
		{Message{Text: ""}, Everyone},
		{Message{Text: "a\nsay hi"}, Everyone},
		{Message{Text: strings.Repeat("a", maxMessage+1)}, Everyone},
		{Message{Kind: "chat", Text: "Hi", Subtitle: "no"}, Everyone},
		{Message{Kind: "bossbar", Text: "Hi"}, Everyone},
		{Message{Text: "Hi"}, "@p"},
		{Message{Text: "Hi"}, "Alex @a"},
		// Many characters that JSON escapes make a command too long for the console.
		{Message{Kind: "title", Text: strings.Repeat("<", maxMessage)}, Everyone},
	} {
		if got, err := tc.m.Commands(tc.target); err == nil {
			t.Errorf("%+v to %s = %q, want an error", tc.m, tc.target, got)
		}
	}
}
