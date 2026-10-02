package logs

import (
	"cmp"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/QwikByte/mc-server-manager/internal/logging"
	"github.com/QwikByte/mc-server-manager/internal/master/access"
	"github.com/QwikByte/mc-server-manager/internal/master/auth"
)

const maxErrorBody = 4 << 10

// action is how the requests of a route appear in the log.
type action struct {
	category slog.Attr
	message  string
}

const (
	routeServer  = "/api/nodes/{node}/servers/{id}"
	routeFiles   = routeServer + "/files"
	routeBackups = routeServer + "/backups"
)

// actions are the routes whose requests are logged: all that change something, and downloads.
var actions = map[string]action{
	"POST /api/groups":                                  {logging.Users, "Create group"},
	"PUT /api/groups/{id}":                              {logging.Users, "Change group"},
	"DELETE /api/groups/{id}":                           {logging.Users, "Delete group"},
	"POST /api/users":                                   {logging.Users, "Invite user"},
	"PUT /api/users/{id}":                               {logging.Users, "Change user"},
	"DELETE /api/users/{id}":                            {logging.Users, "Delete user"},
	"POST /api/users/{id}/setup-link":                   {logging.Users, "Create setup link"},
	"DELETE /api/users/{id}/mfa":                        {logging.Users, "Turn off two-factor authentication"},
	"PUT /api/settings":                                 {logging.Settings, "Change settings"},
	"POST /api/update/check":                            {logging.System, "Check for updates"},
	"POST /api/update/master":                           {logging.System, "Update master"},
	"POST /api/update/agents":                           {logging.Nodes, "Update agents"},
	"POST /api/terminal":                                {logging.Terminal, "Run terminal command"},
	"GET /api/logs/export":                              {logging.System, "Export log"},
	"POST /api/nodes":                                   {logging.Nodes, "Add node"},
	"PUT /api/nodes/{id}":                               {logging.Nodes, "Change node"},
	"DELETE /api/nodes/{id}":                            {logging.Nodes, "Remove node"},
	"POST /api/nodes/{id}/join-token":                   {logging.Nodes, "Create join token"},
	"POST /api/nodes/{id}/certificate":                  {logging.Nodes, "Renew node certificate"},
	"POST /api/nodes/{node}/servers":                    {logging.Servers, "Create server"},
	"PUT " + routeServer:                                {logging.Servers, "Change server settings"},
	"DELETE " + routeServer:                             {logging.Servers, "Delete server"},
	"POST " + routeServer + "/start":                    {logging.Servers, "Start server"},
	"POST " + routeServer + "/stop":                     {logging.Servers, "Stop server"},
	"POST " + routeServer + "/restart":                  {logging.Servers, "Restart server"},
	"POST " + routeServer + "/duplicate":                {logging.Servers, "Duplicate server"},
	"POST " + routeServer + "/command":                  {logging.Console, "Send console command"},
	"PUT " + routeServer + "/properties":                {logging.Files, "Change server.properties"},
	"GET " + routeFiles + "/content":                    {logging.Files, "Download file"},
	"GET " + routeFiles + "/archive":                    {logging.Files, "Download folder"},
	"PUT " + routeFiles + "/content":                    {logging.Files, "Upload file"},
	"POST " + routeFiles + "/directories":               {logging.Files, "Create folder"},
	"POST " + routeFiles + "/move":                      {logging.Files, "Move file"},
	"DELETE " + routeFiles:                              {logging.Files, "Delete file"},
	"POST /api/plugins/install":                         {logging.Plugins, "Install plugins"},
	"PUT " + routeServer + "/plugins/{file}":            {logging.Plugins, "Upload plugin"},
	"DELETE " + routeServer + "/plugins/{file}":         {logging.Plugins, "Remove plugin"},
	"POST " + routeBackups:                              {logging.Backups, "Back up server"},
	"POST " + routeBackups + "/{backup}/restore":        {logging.Backups, "Restore backup"},
	"DELETE " + routeBackups + "/{backup}":              {logging.Backups, "Delete backup"},
	"GET " + routeBackups + "/{backup}/download":        {logging.Backups, "Download backup"},
	"POST /api/backup-jobs":                             {logging.Backups, "Create backup job"},
	"PUT /api/backup-jobs/{id}":                         {logging.Backups, "Change backup job"},
	"DELETE /api/backup-jobs/{id}":                      {logging.Backups, "Delete backup job"},
	"POST /api/backup-jobs/{id}/run":                    {logging.Backups, "Run backup job"},
	"POST /api/policies":                                {logging.Policies, "Create policy"},
	"PUT /api/policies/{id}":                            {logging.Policies, "Change policy"},
	"DELETE /api/policies/{id}":                         {logging.Policies, "Delete policy"},
	"POST /api/policies/{id}/run":                       {logging.Policies, "Run policy"},
	"POST /api/networks":                                {logging.Networks, "Create network"},
	"DELETE /api/networks/{id}":                         {logging.Networks, "Delete network"},
	"POST /api/networks/{id}/apply":                     {logging.Networks, "Apply network"},
	"POST /api/networks/{id}/backends":                  {logging.Networks, "Add server to network"},
	"DELETE /api/networks/{id}/backends/{server}":       {logging.Networks, "Remove server from network"},
	"POST /api/networks/{id}/backends/{server}/default": {logging.Networks, "Change default server of network"},
	"POST /api/templates":                               {logging.Templates, "Create template"},
	"PUT /api/templates/{id}":                           {logging.Templates, "Change template"},
	"DELETE /api/templates/{id}":                        {logging.Templates, "Delete template"},
}

var wildcard = regexp.MustCompile(`\{(\w+)\}`)

// Audit returns the wrapper that logs the requests of the actions: who made them, from
// where, about which node and server, and how they ended. Handlers can add details with
// logging.Note. It panics for routes that change something but have no action, so none is
// left out.
func (s *Store) Audit() access.Wrapper {
	names := s.names
	return func(pattern string, h http.HandlerFunc) http.HandlerFunc {
		a, ok := actions[pattern]
		if method, _, _ := strings.Cut(pattern, " "); !ok && method != http.MethodGet {
			panic("logs: the route " + pattern + " has no action")
		}
		if !ok {
			return h
		}
		return func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			attrs := append(target(r, pattern, names), a.category, slog.String("ip", auth.ClientIP(r)))
			if user, ok := auth.UserFrom(r.Context()); ok {
				attrs = append(attrs, slog.String(logging.KeyUser, user.Username))
			}
			ctx, notes := logging.WithNotes(r.Context())
			rec := &recorder{ResponseWriter: w}
			h(rec, r.WithContext(ctx))

			status, level, message := cmp.Or(rec.status, http.StatusOK), slog.LevelInfo, a.message
			switch {
			case status == http.StatusUnauthorized || status == http.StatusForbidden:
				level, message = slog.LevelWarn, message+" denied"
			case status >= http.StatusInternalServerError:
				level, message = slog.LevelError, message+" failed"
			case status >= http.StatusBadRequest:
				level, message = slog.LevelWarn, message+" failed"
			}
			attrs = append(attrs, slog.Int("status", status), slog.Duration("duration", time.Since(start).Round(time.Millisecond)))
			if status >= http.StatusBadRequest {
				var body struct {
					Error string `json:"error"`
				}
				_ = json.Unmarshal(rec.body, &body)
				attrs = append(attrs, slog.String("err", body.Error))
			}
			slog.LogAttrs(context.WithoutCancel(ctx), level, message, append(attrs, notes()...)...)
		}
	}
}

// target returns the node and the server a request is about with their names, which are
// looked up first, as the request may delete them, and its other path values, e.g. the
// ID of a group, and the path of a file.
func target(r *http.Request, pattern string, names *Names) []slog.Attr {
	var attrs []slog.Attr
	nodeID, serverID := r.PathValue("node"), ""
	for _, m := range wildcard.FindAllStringSubmatch(pattern, -1) {
		name, value := m[1], r.PathValue(m[1])
		switch {
		case name == "node":
		case name == "id" && nodeID != "":
			serverID = value
		case name == "id" && strings.Contains(pattern, " /api/nodes/{id}"):
			nodeID = value
		case name == logging.KeyServer: // a server of a network, whose node the route doesn't name
			attrs = append(attrs, slog.String("server_id", value))
		default:
			attrs = append(attrs, slog.String(name, value))
		}
	}
	if path := r.URL.Query().Get("path"); path != "" {
		attrs = append(attrs, slog.String("path", path))
	}
	if nodeID != "" {
		attrs = append(attrs, slog.String(logging.KeyNode, nodeID), slog.String(logging.KeyNodeName, names.Node(r.Context(), nodeID)))
	}
	if serverID != "" {
		attrs = append(attrs, slog.String(logging.KeyServer, serverID), slog.String(logging.KeyServerName, names.Server(r.Context(), nodeID, serverID)))
	}
	return attrs
}

// recorder keeps the status of a response and the start of an error's body.
type recorder struct {
	http.ResponseWriter
	status int
	body   []byte
}

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	if r.status >= http.StatusBadRequest {
		r.body = append(r.body, p[:min(len(p), maxErrorBody-len(r.body))]...)
	}
	return r.ResponseWriter.Write(p)
}

// Unwrap lets http.ResponseController reach the writer, e.g. to flush a stream.
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
