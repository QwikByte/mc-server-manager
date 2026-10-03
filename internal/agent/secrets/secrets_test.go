package secrets

import (
	"slices"
	"testing"
)

func TestRedactAndRestore(t *testing.T) {
	for _, tc := range []struct{ name, data, redacted string }{
		{"server.properties", "motd=hi\nrcon.password=s3cret\r\nrcon.port=25575\nmanagement-server-secret = abc def \n",
			"motd=hi\nrcon.password=<hidden>\r\nrcon.port=25575\nmanagement-server-secret = <hidden> \n"},
		{"server.properties", "rcon.password=\n", "rcon.password=\n"}, // nothing to hide
		{"config/paper-global.yml", "proxies:\n  velocity:\n    enabled: true\n    secret: 'F0rward1ngS3cret'\n",
			"proxies:\n  velocity:\n    enabled: true\n    secret: <hidden>\n"},
		{"plugins/Example/config.yml", "secret: kept\n", "secret: kept\n"},
	} {
		if got := string(Redact(tc.name, []byte(tc.data))); got != tc.redacted {
			t.Errorf("Redact(%s) = %q, want %q", tc.name, got, tc.redacted)
		}
		// Saving what the panel showed keeps the secrets.
		if got := string(Restore(tc.name, []byte(tc.redacted), []byte(tc.data))); got != tc.data {
			t.Errorf("Restore(%s) = %q, want %q", tc.name, got, tc.data)
		}
	}
	// A new secret is taken as is, and a placeholder without a secret never becomes one.
	got := Restore("server.properties", []byte("rcon.password=n3w\nmanagement-server-secret=<hidden>\n"), []byte("rcon.password=old\n"))
	if want := "rcon.password=n3w\nmanagement-server-secret=\n"; string(got) != want {
		t.Errorf("Restore = %q, want %q", got, want)
	}
}

func TestFiles(t *testing.T) {
	for name, want := range map[string][3]bool{ // hidden, redacted, holds secrets
		".rcon-cli.env":           {true, false, true},
		"forwarding.secret":       {true, false, true},
		"server.properties":       {false, true, true},
		"config":                  {false, false, true},
		"config/paper-global.yml": {false, true, true},
		"plugins":                 {false, false, false},
		"world/server.properties": {false, false, false},
	} {
		if got := [3]bool{Hidden(name), Redacted(name), len(Under(name)) > 0}; got != want {
			t.Errorf("%s: hidden, redacted, holds secrets = %v, want %v", name, got, want)
		}
	}
	if got := Under("config"); !slices.Equal(got, []string{"config/paper-global.yml"}) {
		t.Errorf("Under(config) = %q", got)
	}
}
