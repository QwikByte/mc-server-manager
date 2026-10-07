package access

import (
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/QwikByte/noryx/internal/buildinfo"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// The description of the API is generated from the routes registered with Mux, in OpenAPI
// 3.1: their methods, paths and the permissions they need. Request and response schemas can
// follow.

const apiDescription = "The REST API of the Noryx master. Scripts send an API token, which users create on " +
	"their account page in the panel, as `Authorization: Bearer noryx_…`. A route needs the permissions in " +
	"x-permissions, which the token and its user must both have; scoped ones on the node and server of the " +
	"path. Routes without any answer with what the user may see, and check the permissions of what a request " +
	"names. Errors are JSON objects with the message in `error`."

var wildcard = regexp.MustCompile(`\{(\w+)(\.\.\.)?\}`)

// needs returns the permissions need asks for, one after the other as they are granted, of a
// request without path values, query or body; or Administrators. A Need that asks for more
// depending on the request, e.g. its query, names what it always needs.
func needs(pattern string, need Need) []Permission {
	method, _, _ := strings.Cut(pattern, " ")
	r := &http.Request{Method: method, URL: &url.URL{}, Header: http.Header{}, Body: http.NoBody}
	var g Grants
	perms := []Permission{}
	for {
		p, ok := need(r, g)
		if ok || p == "" || slices.Contains(perms, p) {
			return perms
		}
		perms = append(perms, p)
		if p == Administrators {
			g.admin = true
		} else {
			g.add(p, true, nil)
		}
	}
}

// describe serves the description of the API: every route registered with Handle.
func (m Mux) describe(w http.ResponseWriter, _ *http.Request) {
	paths := map[string]map[string]any{}
	for _, rt := range *m.routes {
		method, path, _ := strings.Cut(rt.pattern, " ")
		path = strings.TrimSuffix(path, "{$}")
		params := []any{}
		for _, match := range wildcard.FindAllStringSubmatch(path, -1) {
			params = append(params, map[string]any{"name": match[1], "in": "path", "required": true, "schema": map[string]string{"type": "string"}})
		}
		path = wildcard.ReplaceAllString(path, "{$1}")
		if paths[path] == nil {
			paths[path] = map[string]any{}
		}
		paths[path][strings.ToLower(method)] = map[string]any{
			"description":   describeNeeds(rt.needs),
			"parameters":    params,
			"x-permissions": rt.needs,
			"responses":     map[string]any{"default": map[string]string{"description": "JSON, or no content"}},
		}
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"openapi":    "3.1.0",
		"info":       map[string]string{"title": "Noryx", "version": buildinfo.Version, "description": apiDescription},
		"components": map[string]any{"securitySchemes": map[string]any{"token": map[string]string{"type": "http", "scheme": "bearer"}}},
		"security":   []map[string][]string{{"token": {}}},
		"paths":      paths,
	})
}

func describeNeeds(perms []Permission) string {
	switch {
	case len(perms) == 0:
		return "Needs no permission of its own."
	case perms[0] == Administrators:
		return "Only administrators may use it."
	}
	names := make([]string, len(perms))
	for i, p := range perms {
		names[i] = string(p)
	}
	return "Needs " + strings.Join(names, ", ") + "."
}
