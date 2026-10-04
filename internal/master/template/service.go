// Package template stores templates for new servers: their settings, server.properties
// and plugins. Plugins are Modrinth projects; the version that suits a new server is
// installed when the server is created from the template, so templates don't go stale.
package template

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"

	noryxv1 "github.com/QwikByte/mc-server-manager/api/noryx/v1"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
	"github.com/QwikByte/mc-server-manager/internal/master/modrinth"
	"github.com/QwikByte/mc-server-manager/internal/master/plugin"
)

const (
	maxDescription = 500
	maxProperties  = 200
	maxPropValue   = 4096
	maxJVMOptions  = 32
	maxPlugins     = 50
)

var (
	errNotFound     = httpapi.Errorf(http.StatusNotFound, "Template not found.")
	versionPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,32}$`)
	propertyPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)
)

type Template struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Settings
	Plugins   []plugin.Project `json:"plugins"`
	CreatedAt time.Time        `json:"createdAt"`
}

// Settings are the settings and server.properties of the servers created from a template.
// The agent validates them in detail when a server is created.
type Settings struct {
	Type          string            `json:"type"`
	Version       string            `json:"version"` // LATEST follows new releases
	MemoryMB      uint32            `json:"memoryMb"`
	Java          string            `json:"java"`
	RestartPolicy string            `json:"restartPolicy"`
	AikarFlags    bool              `json:"aikarFlags"`
	JVMOptions    []string          `json:"jvmOptions"`
	CPULimit      float64           `json:"cpuLimit"` // in cores, 0 means no limit
	Properties    map[string]string `json:"properties"`
}

// Input is a new or changed template; plugins are Modrinth project IDs.
type Input struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Settings
	Plugins []string `json:"plugins"`
}

// Plugins looks up Modrinth projects.
type Plugins interface {
	Projects(ctx context.Context, ids []string) ([]modrinth.Project, error)
	Describe(p modrinth.Project) plugin.Project
}

type Service struct {
	db      *sql.DB
	plugins Plugins
}

func NewService(db *sql.DB, plugins Plugins) *Service { return &Service{db: db, plugins: plugins} }

func (s *Service) List(ctx context.Context) ([]Template, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, description, settings, created_at FROM templates ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	templates := []Template{}
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, rows.Err()
}

func (s *Service) Get(ctx context.Context, id string) (Template, error) {
	t, err := scan(s.db.QueryRowContext(ctx, `SELECT id, name, description, settings, created_at FROM templates WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		err = errNotFound
	}
	return t, err
}

func (s *Service) Create(ctx context.Context, in Input) (Template, error) {
	t, err := s.build(ctx, in)
	if err != nil {
		return t, err
	}
	t.ID, t.CreatedAt = strings.ToLower(rand.Text()), time.Now()
	data, err := t.data()
	if err == nil {
		_, err = s.db.ExecContext(ctx, `INSERT INTO templates (id, name, description, settings, created_at) VALUES (?, ?, ?, ?, ?)`,
			t.ID, t.Name, t.Description, data, t.CreatedAt.Unix())
	}
	return t, uniqueName(err, t.Name)
}

func (s *Service) Update(ctx context.Context, id string, in Input) (Template, error) {
	t, err := s.build(ctx, in)
	if err != nil {
		return t, err
	}
	data, err := t.data()
	if err != nil {
		return t, err
	}
	res, err := s.db.ExecContext(ctx, `UPDATE templates SET name = ?, description = ?, settings = ? WHERE id = ?`, t.Name, t.Description, data, id)
	if err != nil {
		return t, uniqueName(err, t.Name)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return t, errNotFound
	}
	return s.Get(ctx, id)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM templates WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errNotFound
	}
	return nil
}

// build validates a template and looks up its plugins, which must support its type.
func (s *Service) build(ctx context.Context, in Input) (Template, error) {
	t := Template{Name: strings.TrimSpace(in.Name), Description: strings.TrimSpace(in.Description), Settings: in.Settings, Plugins: []plugin.Project{}}
	typ := noryxv1.ParseServerType(t.Type)
	t.Type = typ.Slug()
	if typ.Proxy() {
		t.Version, t.Java, t.AikarFlags = "LATEST", "", false
	} else if t.Version == "" {
		t.Version = "LATEST"
	}
	t.RestartPolicy = noryxv1.ParseRestartPolicy(t.RestartPolicy).Slug()
	t.JVMOptions = append([]string{}, t.JVMOptions...)
	if t.Properties == nil {
		t.Properties = map[string]string{}
	}
	if msg := check(t, typ, in); msg != "" {
		return t, httpapi.Errorf(http.StatusBadRequest, "%s", msg)
	}
	ids := slices.Compact(slices.Sorted(slices.Values(in.Plugins)))
	if len(ids) == 0 {
		return t, nil
	}
	projects, err := s.plugins.Projects(ctx, ids)
	if err != nil {
		return t, err
	}
	for _, id := range ids {
		i := slices.IndexFunc(projects, func(p modrinth.Project) bool { return p.ID == id })
		if i < 0 {
			return t, httpapi.Errorf(http.StatusBadRequest, "The project %s doesn't exist on Modrinth.", id)
		}
		if !slices.ContainsFunc(modrinth.Loaders(typ), func(l string) bool { return slices.Contains(projects[i].Loaders, l) }) {
			return t, httpapi.Errorf(http.StatusBadRequest, "%s doesn't run on %s servers.", projects[i].Title, t.Type)
		}
		t.Plugins = append(t.Plugins, s.plugins.Describe(projects[i]))
	}
	return t, nil
}

// check returns a message for the administrator if a template is invalid.
func check(t Template, typ noryxv1.ServerType, in Input) string {
	switch {
	case t.Name == "" || len(t.Name) > 64:
		return "Enter a name with up to 64 characters."
	case len(t.Description) > maxDescription:
		return "Keep the description below 500 characters."
	case typ == noryxv1.ServerType_SERVER_TYPE_UNSPECIFIED:
		return "Choose the software of the servers."
	case !versionPattern.MatchString(t.Version):
		return "Enter a Minecraft version like 1.21.4, or leave it empty for the latest."
	case t.MemoryMB < 512 || t.MemoryMB > 64*1024:
		return "Memory must be between 512 and 65536 MB."
	case in.RestartPolicy != "" && noryxv1.ParseRestartPolicy(in.RestartPolicy) == noryxv1.RestartPolicy_RESTART_POLICY_UNSPECIFIED:
		return "Choose when the servers start on their own."
	case t.CPULimit < 0 || t.CPULimit > 1024:
		return "Enter a CPU limit in cores, or 0 for no limit."
	case len(t.JVMOptions) > maxJVMOptions:
		return "Use at most 32 JVM options."
	case typ.Proxy() && len(t.Properties) > 0:
		return "Proxies have no server.properties."
	case len(t.Properties) > maxProperties:
		return "Use at most 200 properties."
	case len(in.Plugins) > maxPlugins:
		return "Use at most 50 plugins."
	}
	for key, value := range t.Properties {
		if !propertyPattern.MatchString(key) || len(value) > maxPropValue ||
			strings.ContainsFunc(value, func(r rune) bool { return r != '\n' && unicode.IsControl(r) }) {
			return "The property " + key + " is invalid."
		}
	}
	for _, id := range in.Plugins {
		if !modrinth.ValidProjectID(id) {
			return "Invalid plugin " + id + "."
		}
	}
	return ""
}

// stored is the JSON in the settings column.
type stored struct {
	Settings
	Plugins []plugin.Project `json:"plugins"`
}

func (t Template) data() (string, error) {
	data, err := json.Marshal(stored{t.Settings, t.Plugins})
	return string(data), err
}

type scanner interface{ Scan(dest ...any) error }

func scan(row scanner) (Template, error) {
	var t Template
	var data string
	var createdAt int64
	if err := row.Scan(&t.ID, &t.Name, &t.Description, &data, &createdAt); err != nil {
		return t, err
	}
	var s stored
	if err := json.Unmarshal([]byte(data), &s); err != nil {
		return t, err
	}
	t.Settings, t.Plugins, t.CreatedAt = s.Settings, s.Plugins, time.Unix(createdAt, 0)
	return t, nil
}

func uniqueName(err error, name string) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return httpapi.Errorf(http.StatusConflict, "A template named %q already exists.", name)
	}
	return err
}
