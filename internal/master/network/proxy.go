package network

import (
	"context"
	"encoding/json"
	"net/http"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

// ProxySettings are the settings of a proxy in its own configuration file, by their path in
// the file, e.g. "advanced.compression-threshold", as JSON values.
type ProxySettings struct {
	// Exists is false until the proxy started once and created its configuration.
	Exists   bool                       `json:"exists"`
	File     string                     `json:"file"`
	Settings map[string]json.RawMessage `json:"settings"`
	// Locked are the settings the manager or the network decides, with the reason.
	Locked []Locked `json:"locked"`
}

type Locked struct {
	Key    string `json:"key"`
	Reason string `json:"reason"`
}

func (h *Handler) proxySettings(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
	defer cancel()
	c, err := h.proxyClient(ctx, r)
	var res *noryxv1.GetProxySettingsResponse
	if err == nil {
		res, err = c.GetProxySettings(ctx, &noryxv1.GetProxySettingsRequest{ServerId: r.PathValue("id")})
	}
	if err != nil {
		httpapi.WriteError(w, r, err)
		return
	}
	view := ProxySettings{Exists: res.GetExists(), File: res.GetFile(), Settings: map[string]json.RawMessage{}, Locked: []Locked{}}
	for key, value := range res.GetSettings() {
		view.Settings[key] = json.RawMessage(value)
	}
	for _, l := range res.GetLocked() {
		view.Locked = append(view.Locked, Locked{l.GetKey(), l.GetReason()})
	}
	httpapi.WriteJSON(w, http.StatusOK, view)
}

// updateProxySettings changes settings of a proxy, which reloads its configuration if it runs.
func (h *Handler) updateProxySettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Settings map[string]json.RawMessage `json:"settings"`
	}
	if !read(w, r, &req) {
		return
	}
	settings := make(map[string]string, len(req.Settings))
	for key, value := range req.Settings {
		settings[key] = string(value)
	}
	// Reloading waits for the proxy's answer.
	ctx, cancel := context.WithTimeout(r.Context(), configureTimeout)
	defer cancel()
	c, err := h.proxyClient(ctx, r)
	var res *noryxv1.UpdateProxySettingsResponse
	if err == nil {
		res, err = c.UpdateProxySettings(ctx, &noryxv1.UpdateProxySettingsRequest{ServerId: r.PathValue("id"), Settings: settings})
	}
	write(w, r, http.StatusOK, map[string]bool{"reloaded": res.GetReloaded()}, err)
}

func (h *Handler) proxyClient(ctx context.Context, r *http.Request) (noryxv1.ProxyServiceClient, error) {
	conn, err := h.svc.nodes.Conn(ctx, r.PathValue("node"))
	if err != nil {
		return nil, err
	}
	return noryxv1.NewProxyServiceClient(conn), nil
}
