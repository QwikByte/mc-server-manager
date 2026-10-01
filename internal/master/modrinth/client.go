// Package modrinth is a client for the API of Modrinth (https://modrinth.com), the
// catalogue the panel installs plugins and mods from. Files are only downloaded from
// Modrinth's CDN and only used if their SHA-512 hash matches the one of the API.
package modrinth

import (
	"bytes"
	"cmp"
	"context"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
	"github.com/QwikByte/mc-server-manager/internal/master/httpapi"
)

const (
	DefaultAPI = "https://api.modrinth.com/v2"
	DefaultCDN = "https://cdn.modrinth.com/"
	// MaxFileSize matches the limit of the agents.
	MaxFileSize      = 256 << 20
	maxResponseBytes = 16 << 20
	maxIconBytes     = 1 << 20
	pageSize         = 20
	releaseTTL       = time.Hour
)

var (
	// loaders run the plugins or mods of a server type, the most specific first.
	loaders = map[mcsmv1.ServerType][]string{
		mcsmv1.ServerType_SERVER_TYPE_PAPER:      {"paper", "spigot", "bukkit"},
		mcsmv1.ServerType_SERVER_TYPE_PURPUR:     {"purpur", "paper", "spigot", "bukkit"},
		mcsmv1.ServerType_SERVER_TYPE_VELOCITY:   {"velocity"},
		mcsmv1.ServerType_SERVER_TYPE_BUNGEECORD: {"bungeecord", "waterfall"},
		mcsmv1.ServerType_SERVER_TYPE_FABRIC:     {"fabric"},
		mcsmv1.ServerType_SERVER_TYPE_FORGE:      {"forge"},
		mcsmv1.ServerType_SERVER_TYPE_NEOFORGE:   {"neoforge"},
	}
	projectID = regexp.MustCompile(`^[A-Za-z0-9]{1,32}$`)
	// iconName matches the path of a project icon on the CDN, e.g. AABBCCDD/icon.png.
	iconName   = regexp.MustCompile(`^[A-Za-z0-9]{1,32}/[A-Za-z0-9_-]{1,128}\.(png|jpe?g|webp|gif)$`)
	iconTypes  = map[string]string{"png": "image/png", "jpg": "image/jpeg", "jpeg": "image/jpeg", "webp": "image/webp", "gif": "image/gif"}
	errUnknown = httpapi.Errorf(http.StatusNotFound, "This project doesn't exist on Modrinth.")
)

// Loaders returns the loaders whose plugins or mods run on a server type; none for
// vanilla servers.
func Loaders(t mcsmv1.ServerType) []string { return loaders[t] }

// AllLoaders returns the loaders of all server types.
func AllLoaders() []string {
	var all []string
	for _, names := range loaders {
		all = append(all, names...)
	}
	slices.Sort(all)
	return slices.Compact(all)
}

// ValidProjectID reports whether id is a well-formed project ID.
func ValidProjectID(id string) bool { return projectID.MatchString(id) }

type Project struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	IconURL     string   `json:"icon_url"`
	Downloads   int64    `json:"downloads"`
	Loaders     []string `json:"loaders"`
}

type SearchResult struct {
	Hits  []SearchHit `json:"hits"`
	Total int         `json:"total_hits"`
}

type SearchHit struct {
	ProjectID   string   `json:"project_id"`
	Slug        string   `json:"slug"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	IconURL     string   `json:"icon_url"`
	Downloads   int64    `json:"downloads"`
	Categories  []string `json:"categories"` // includes the loaders
}

type Version struct {
	ID            string       `json:"id"`
	ProjectID     string       `json:"project_id"`
	VersionNumber string       `json:"version_number"`
	VersionType   string       `json:"version_type"` // release, beta or alpha
	Published     time.Time    `json:"date_published"`
	Files         []File       `json:"files"`
	Dependencies  []Dependency `json:"dependencies"`
}

// File returns the primary file of a version.
func (v Version) File() (File, bool) {
	if len(v.Files) == 0 {
		return File{}, false
	}
	return v.Files[max(0, slices.IndexFunc(v.Files, func(f File) bool { return f.Primary }))], true
}

type File struct {
	URL      string `json:"url"`
	Filename string `json:"filename"`
	Primary  bool   `json:"primary"`
	Size     int64  `json:"size"`
	Hashes   struct {
		SHA512 string `json:"sha512"`
	} `json:"hashes"`
}

type Dependency struct {
	ProjectID string `json:"project_id"`
	Type      string `json:"dependency_type"` // required, optional, incompatible or embedded
}

type Client struct {
	api, cdn string
	http     *http.Client

	mu        sync.Mutex
	release   string // the newest Minecraft release
	releaseAt time.Time
}

// New returns a client for the API and CDN at the given base URLs.
func New(api, cdn string) *Client {
	return &Client{api: api, cdn: cdn, http: &http.Client{Timeout: 2 * time.Minute}}
}

// Search finds server-side plugins and mods for the given loaders, all of them if none
// are given, and optionally a Minecraft version. Without a query, the most downloaded
// projects come first.
func (c *Client) Search(ctx context.Context, query string, loaderNames []string, gameVersion string, offset int) (SearchResult, error) {
	if len(loaderNames) == 0 {
		loaderNames = AllLoaders()
	}
	facets := [][]string{
		prefixed("categories:", loaderNames),
		{"server_side:required", "server_side:optional"},
		{"project_type:plugin", "project_type:mod"},
	}
	if gameVersion != "" {
		facets = append(facets, []string{"versions:" + gameVersion})
	}
	q := url.Values{"query": {query}, "facets": {jsonText(facets)}, "limit": {strconv.Itoa(pageSize)}, "offset": {strconv.Itoa(offset)}}
	if query == "" {
		q.Set("index", "downloads")
	}
	var res SearchResult
	return res, c.call(ctx, http.MethodGet, "/search?"+q.Encode(), nil, &res)
}

// Projects returns the projects with the given IDs; unknown ones are left out.
func (c *Client) Projects(ctx context.Context, ids []string) ([]Project, error) {
	projects := []Project{}
	if len(ids) == 0 {
		return projects, nil
	}
	return projects, c.call(ctx, http.MethodGet, "/projects?"+url.Values{"ids": {jsonText(ids)}}.Encode(), nil, &projects)
}

// Versions returns the versions of a project for the given loaders and, unless empty,
// Minecraft version, the newest first.
func (c *Client) Versions(ctx context.Context, project string, loaderNames []string, gameVersion string) ([]Version, error) {
	if !ValidProjectID(project) {
		return nil, errUnknown
	}
	q := url.Values{"loaders": {jsonText(loaderNames)}, "include_changelog": {"false"}}
	if gameVersion != "" {
		q.Set("game_versions", jsonText([]string{gameVersion}))
	}
	var versions []Version
	err := c.call(ctx, http.MethodGet, "/project/"+project+"/version?"+q.Encode(), nil, &versions)
	slices.SortStableFunc(versions, func(a, b Version) int { return b.Published.Compare(a.Published) })
	return versions, err
}

// VersionsByHash identifies files by their SHA-512 hashes. Unknown files are left out.
func (c *Client) VersionsByHash(ctx context.Context, hashes []string) (map[string]Version, error) {
	versions := map[string]Version{}
	return versions, c.call(ctx, http.MethodPost, "/version_files", map[string]any{"hashes": hashes, "algorithm": "sha512"}, &versions)
}

// Updates returns the newest release for the given loaders and, unless empty, Minecraft
// version of the projects that the files with the given hashes belong to.
func (c *Client) Updates(ctx context.Context, hashes, loaderNames []string, gameVersion string) (map[string]Version, error) {
	body := map[string]any{"hashes": hashes, "algorithm": "sha512", "loaders": loaderNames, "version_types": []string{"release"}}
	if gameVersion != "" {
		body["game_versions"] = []string{gameVersion}
	}
	versions := map[string]Version{}
	return versions, c.call(ctx, http.MethodPost, "/version_files/update", body, &versions)
}

// LatestRelease returns the newest release of Minecraft.
func (c *Client) LatestRelease(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Since(c.releaseAt) < releaseTTL {
		return c.release, nil
	}
	var versions []gameVersion
	if err := c.call(ctx, http.MethodGet, "/tag/game_version", nil, &versions); err != nil {
		return "", err
	}
	i := slices.IndexFunc(versions, func(v gameVersion) bool { return v.Type == "release" })
	if i < 0 {
		return "", httpapi.Errorf(http.StatusBadGateway, "Modrinth lists no Minecraft release.")
	}
	c.release, c.releaseAt = versions[i].Version, time.Now()
	return c.release, nil
}

// gameVersion is a Minecraft version, the newest first.
type gameVersion struct {
	Version string `json:"version"`
	Type    string `json:"version_type"` // release or snapshot
}

// Download fetches a file from the CDN and checks its size and hash.
func (c *Client) Download(ctx context.Context, f File) ([]byte, error) {
	if !strings.HasPrefix(f.URL, c.cdn) || f.Size > MaxFileSize || len(f.Hashes.SHA512) != sha512.Size*2 {
		return nil, httpapi.Errorf(http.StatusBadGateway, "%s can't be downloaded: it isn't on Modrinth's CDN or is larger than %d MB.", f.Filename, MaxFileSize>>20)
	}
	data, err := c.fetch(ctx, f.URL, MaxFileSize)
	if err != nil {
		return nil, err
	}
	if sum := sha512.Sum512(data); hex.EncodeToString(sum[:]) != strings.ToLower(f.Hashes.SHA512) {
		return nil, httpapi.Errorf(http.StatusBadGateway, "The download of %s is corrupted. Try again.", f.Filename)
	}
	return data, nil
}

// IconName returns the name of a project icon on the CDN, e.g. AABBCCDD/icon.png, or ""
// for icons elsewhere.
func (c *Client) IconName(iconURL string) string {
	name, ok := strings.CutPrefix(iconURL, c.cdn+"data/")
	if !ok || !iconName.MatchString(name) {
		return ""
	}
	return name
}

// Icon fetches a project icon by its name, see IconName, and returns its media type.
func (c *Client) Icon(ctx context.Context, name string) ([]byte, string, error) {
	if !iconName.MatchString(name) {
		return nil, "", httpapi.Errorf(http.StatusNotFound, "Icon not found.")
	}
	data, err := c.fetch(ctx, c.cdn+"data/"+name, maxIconBytes)
	return data, iconTypes[name[strings.LastIndexByte(name, '.')+1:]], err
}

// call sends a request to the API and decodes its JSON response into out.
func (c *Client) call(ctx context.Context, method, path string, body, out any) error {
	var r io.Reader
	if body != nil {
		r = strings.NewReader(jsonText(body))
	}
	res, err := c.do(ctx, method, c.api+path, r)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if err := json.NewDecoder(io.LimitReader(res.Body, maxResponseBytes)).Decode(out); err != nil {
		return unavailable(err)
	}
	return nil
}

// fetch downloads a file of up to limit bytes.
func (c *Client) fetch(ctx context.Context, fileURL string, limit int64) ([]byte, error) {
	res, err := c.do(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, io.LimitReader(res.Body, limit+1)); err != nil {
		return nil, unavailable(err)
	}
	if int64(buf.Len()) > limit {
		return nil, httpapi.Errorf(http.StatusBadGateway, "The file is larger than %d MB.", limit>>20)
	}
	return buf.Bytes(), nil
}

func (c *Client) do(ctx context.Context, method, target string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, err
	}
	// Modrinth asks clients to identify themselves.
	req.Header.Set("User-Agent", "QwikByte/mc-server-manager/"+buildinfo.Version+" (github.com/QwikByte/mc-server-manager)")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := c.http.Do(req)
	if err != nil {
		return nil, unavailable(cmp.Or(ctx.Err(), err))
	}
	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		if res.StatusCode == http.StatusNotFound {
			return nil, errUnknown
		}
		return nil, unavailable(fmt.Errorf("status %d", res.StatusCode))
	}
	return res, nil
}

func unavailable(err error) error {
	return httpapi.Errorf(http.StatusBadGateway, "Modrinth can't be reached right now (%v). Try again later.", err)
}

func prefixed(prefix string, values []string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = prefix + v
	}
	return out
}

func jsonText(v any) string {
	data, _ := json.Marshal(v) //nolint:errchkjson // only strings, slices and maps of them
	return string(data)
}
