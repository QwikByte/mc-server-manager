package enrollment

import "testing"

func TestTokenRoundTrip(t *testing.T) {
	want := Token{Master: "panel.example.com:9443", NodeID: "n1", Secret: "s3cret", CAFingerprint: "ab12"}
	got, err := ParseToken(" " + want.String() + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestParseTokenRejectsGarbage(t *testing.T) {
	for _, s := range []string{"", "noryx1_", "noryx1_!!!", Token{Master: "x"}.String()} {
		if _, err := ParseToken(s); err == nil {
			t.Errorf("ParseToken(%q) succeeded", s)
		}
	}
}
