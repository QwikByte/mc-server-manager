package overlay

import (
	"strings"
	"testing"
)

// The comment of a rule is read from user data of any shape that someone on the node could write.
func TestComment(t *testing.T) {
	long := strings.Repeat("x", 254)
	for data, want := range map[string]string{
		"\x00\x05text\x00":         "text",
		"\x01\x02ab\x00\x03hi\x00": "hi",
		"\x00\xfe" + long:          long,
		"\x01\xffab":               "",
		"\x00\xff":                 "",
		"\x00":                     "",
		"":                         "",
	} {
		if got := comment([]byte(data)); got != want {
			t.Errorf("comment(%q) = %q, want %q", data, got, want)
		}
	}
}
