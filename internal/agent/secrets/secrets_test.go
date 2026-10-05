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
		{"config/FabricProxy-Lite.toml", "hackOnlineMode = true\nsecret = 'F0rward1ngS3cret'\n", "hackOnlineMode = true\nsecret = <hidden>\n"},
		{"config/proxy-compatible-forge.toml", "[forwarding]\nenabled = true\nsecret = 'F0rward1ngS3cret'\n", "[forwarding]\nenabled = true\nsecret = <hidden>\n"},
		{"plugins/Example/config.yml", "secret: kept\n", "secret: kept\n"},
		{"plugins/Geyser-Velocity/config.yml", "bedrock:\n  port: 19132\n  signaling:\n    nxs:\n      token: 'abc'\n", "bedrock:\n  port: 19132\n  signaling:\n    nxs:\n      token: <hidden>\n"},
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
		".rcon-cli.env":                        {true, false, true},
		"forwarding.secret":                    {true, false, true},
		"server.properties":                    {false, true, true},
		"config":                               {false, false, true},
		"config/paper-global.yml":              {false, true, true},
		"plugins":                              {false, false, true}, // Floodgate's key
		"plugins/floodgate/key.pem":            {true, false, true},
		"plugins/Geyser-BungeeCord/config.yml": {false, true, true},
		"plugins/Example":                      {false, false, false},
		"world/server.properties":              {false, false, false},
	} {
		if got := [3]bool{Hidden(name), Redacted(name), len(Under(name)) > 0}; got != want {
			t.Errorf("%s: hidden, redacted, holds secrets = %v, want %v", name, got, want)
		}
	}
	if got := slices.Sorted(slices.Values(Under("config"))); !slices.Equal(got, []string{"config/FabricProxy-Lite.toml", "config/paper-global.yml", "config/proxy-compatible-forge.toml"}) {
		t.Errorf("Under(config) = %q", got)
	}
}
