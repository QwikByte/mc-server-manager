package fileset

import (
	"errors"
	"strings"
	"testing"

	"github.com/QwikByte/noryx/internal/master/tag"
)

func TestCheck(t *testing.T) {
	in := Input{
		Name: " LuckPerms ",
		Files: []File{
			{Path: "/plugins/LuckPerms/config.yml", Content: "server: {{network.server}}\npassword: {{secret:db}}\n"},
			{Path: "bukkit.yml", Content: "{{player}} stays as it is"},
			{Path: "plugins/CoreProtect/config.yml", Content: "host: {{datastore:main.coreprotect.host}}\npassword: {{datastore:main.coreprotect.password}}\n"},
		},
		Targets: []Target{
			{Kind: KindTag, Value: " Lobby "}, {Kind: KindTag, Value: "lobby"},
			{Kind: KindNetwork, Value: "n1", Role: RoleProxy}, {Kind: KindNetwork, Value: "gone", Role: RoleServers},
		},
	}
	if err := in.check(func(id string) bool { return id == "n1" }); err != nil {
		t.Fatal(err)
	}
	if in.Name != "LuckPerms" || in.Files[0].Path != "bukkit.yml" || in.Files[2].Path != "plugins/LuckPerms/config.yml" {
		t.Errorf("input = %+v", in)
	}
	// Tags are normalized, and targets of deleted networks left out.
	if len(in.Targets) != 2 || in.Targets[0] != (Target{Kind: KindNetwork, Value: "n1", Role: RoleProxy}) || in.Targets[1] != (Target{Kind: KindTag, Value: "lobby"}) {
		t.Errorf("targets = %+v", in.Targets)
	}

	for name, f := range map[string]File{
		"managed":          {Path: "spigot.yml"},
		"banned players":   {Path: "banned-players.json"},
		"unknown variable": {Path: "a.yml", Content: "{{server.ip}}"},
		"datastore field":  {Path: "a.yml", Content: "{{datastore:main.lp.secret}}"},
		"datastore name":   {Path: "a.yml", Content: "{{datastore:Main.lp.password}}"},
		"bad secret":       {Path: "a.yml", Content: "{{secret:Not Valid}}"},
		"binary":           {Path: "a.yml", Content: "\x00"},
		"class":            {Path: "Plugin.CLASS"},
		"folder":           {Path: "plugins/"},
	} {
		bad := Input{Name: "x", Files: []File{f}}
		if err := bad.check(func(string) bool { return true }); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	twice := Input{Name: "x", Files: []File{{Path: "a.yml"}, {Path: "/a.yml"}}}
	if err := twice.check(func(string) bool { return true }); err == nil {
		t.Error("a path twice was accepted")
	}
}

func TestRender(t *testing.T) {
	files := []File{
		{Path: "a.yml", Content: "name: {{server.name}} ({{server.id}}:{{server.port}}) in {{network.server}}\npassword: {{secret:db}}\n"},
	}
	m := member{Server: tag.Server{NodeID: "n1", ServerID: "s1"}, Name: "Lobby 1", Port: 25565, Network: "lobby-1"}
	r, err := render("set", files, map[string]string{"db": "pw", "other": "unused"}, m)
	if err != nil {
		t.Fatal(err)
	}
	// The master fills in the variables, the agent the secrets, of which it gets those used.
	if got, _ := r.shown("a.yml"); got != "name: Lobby 1 (s1:25565) in lobby-1\npassword: {{secret:db}}\n" {
		t.Errorf("rendered = %q", got)
	}
	if len(r.secrets) != 1 || r.secrets["secret:db"] != "pw" {
		t.Errorf("secrets = %v", r.secrets)
	}

	// Another secret or another server is another revision; the same input the same.
	same, _ := render("set", files, map[string]string{"db": "pw"}, m)
	rotated, _ := render("set", files, map[string]string{"db": "new"}, m)
	renamed, _ := render("set", files, map[string]string{"db": "pw"}, member{Server: m.Server, Name: "Lobby 2", Port: 25565, Network: "lobby-1"})
	if same.revision != r.revision || rotated.revision == r.revision || renamed.revision == r.revision {
		t.Error("revisions don't tell the states apart")
	}

	// A missing secret, or a network variable on a server outside networks, is a problem.
	if _, err := render("set", files, nil, m); err == nil || !strings.Contains(err.Error(), "db") {
		t.Errorf("missing secret: %v", err)
	}
	m.Network = ""
	if _, err := render("set", files, map[string]string{"db": "pw"}, m); err == nil {
		t.Error("network.server outside of a network was filled in")
	}
}

func TestRenderDatastores(t *testing.T) {
	files := []File{{Path: "a.yml", Content: "{{datastore:main.lp.host}}:{{datastore:main.lp.port}}/{{datastore:main.lp.database}} {{datastore:main.lp.user}} {{datastore:main.lp.password}}"}}
	fields := map[string]string{"main.lp.host": "noryx-db-x", "main.lp.port": "3306", "main.lp.database": "lp", "main.lp.user": "lp", "main.lp.password": "pw"}
	m := member{Server: tag.Server{NodeID: "n1", ServerID: "s1"}, Name: "Lobby", NetworkID: "net"}
	m.datastore = func(key string) (string, error) {
		if v, ok := fields[key]; ok {
			return v, nil
		}
		return "", errors.New("unreachable")
	}
	r, err := render("set", files, nil, m)
	if err != nil {
		t.Fatal(err)
	}
	// Only the password is a secret, which the agent fills in.
	if got, _ := r.shown("a.yml"); got != "noryx-db-x:3306/lp lp {{datastore:main.lp.password}}" || len(r.secrets) != 1 || r.secrets["datastore:main.lp.password"] != "pw" {
		t.Errorf("rendered %q with %v", got, r.secrets)
	}
	fields["main.lp.password"] = "new"
	if rotated, _ := render("set", files, nil, m); rotated.revision == r.revision {
		t.Error("a new password is the same revision")
	}
	delete(fields, "main.lp.host")
	if _, err := render("set", files, nil, m); err == nil {
		t.Error("an unreachable datastore was filled in")
	}
	m.NetworkID = ""
	if _, err := render("set", files, nil, m); err == nil || !strings.Contains(err.Error(), "no network") {
		t.Errorf("outside of a network: %v", err)
	}
}
