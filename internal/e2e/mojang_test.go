package e2e

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const (
	// alexSkin is the texture of Alex's skin, also the one GeyserMC has of the Bedrock player Tim 203.
	alexSkin = "3b60a1f6d562f52aaebbf1434f1de147933a3affe0e764fa49ea057536623cd3"
	alexID   = "6ab4317889fd490597f60f67d9d76fd9"
	plainID  = "0123456789abcdef0123456789abcdef"
)

// fakeMojang serves the parts of Mojang's API, session server and textures server the master
// uses: Alex has a skin with a red face and a hat with a blue pixel, Plain has the default
// skin, others don't exist.
type fakeMojang struct {
	*httptest.Server
	// lookups counts the profiles looked up.
	lookups atomic.Int64
}

func startMojang(t *testing.T) *fakeMojang {
	f := &fakeMojang{}
	mux := http.NewServeMux()
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	mux.HandleFunc("GET /users/profiles/minecraft/{name}", func(w http.ResponseWriter, r *http.Request) {
		f.lookups.Add(1)
		switch name := r.PathValue("name"); {
		case strings.EqualFold(name, "Alex"):
			writeJSON(w, map[string]string{"id": alexID, "name": "Alex"})
		case strings.EqualFold(name, "Plain"):
			writeJSON(w, map[string]string{"id": plainID, "name": "Plain"})
		default:
			w.WriteHeader(http.StatusNotFound)
			writeJSON(w, map[string]string{"errorMessage": "Couldn't find any profile with name " + name})
		}
	})
	mux.HandleFunc("GET /session/minecraft/profile/{id}", func(w http.ResponseWriter, r *http.Request) {
		textures := map[string]any{"textures": map[string]any{}}
		if r.PathValue("id") == alexID {
			textures["textures"] = map[string]any{"SKIN": map[string]string{"url": "http://textures.minecraft.net/texture/" + alexSkin}}
		}
		data, _ := json.Marshal(textures)
		writeJSON(w, map[string]any{"id": r.PathValue("id"), "properties": []map[string]string{{"name": "textures", "value": base64.StdEncoding.EncodeToString(data)}}})
	})
	mux.HandleFunc("GET /texture/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") != alexSkin {
			http.NotFound(w, r)
			return
		}
		skin := image.NewNRGBA(image.Rect(0, 0, 64, 64))
		for y := 8; y < 16; y++ {
			for x := 8; x < 16; x++ {
				skin.Set(x, y, color.NRGBA{R: 255, A: 255})
			}
		}
		skin.Set(40, 8, color.NRGBA{B: 255, A: 255})
		var buf bytes.Buffer
		_ = png.Encode(&buf, skin)
		_, _ = w.Write(buf.Bytes())
	})
	return f
}
