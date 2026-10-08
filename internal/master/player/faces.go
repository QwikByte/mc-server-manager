package player

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/draw"
	"image/png"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
	"golang.org/x/time/rate"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/master/httpapi"
)

const (
	// faceTime is how long a face is kept, and noFaceTime that a player has none.
	faceTime   = 24 * time.Hour
	noFaceTime = time.Hour
	// maxFaces is the most faces kept, about 200 bytes each.
	maxFaces = 5000
	// Faces are looked up lookupBurst at once, then one every lookupEvery: each takes two
	// requests to Mojang's rate limited API, or to GeyserMC's.
	lookupBurst = 30
	lookupEvery = 2 * time.Second
	lookupWait  = 3 * time.Second
	lookupTime  = 20 * time.Second
)

// ErrNoFace tells that a player has no face of their own, e.g. the default skin.
var ErrNoFace = httpapi.Errorf(http.StatusNotFound, "The player has no face of their own.")

var errBusy = httpapi.Errorf(http.StatusServiceUnavailable, "Too many faces of players are looked up right now. Try again later.")

// Skins finds the skins of players on Minecraft's textures server: the ID of a player's skin,
// or "" without one of their own, and the skin.
type Skins interface {
	SkinTexture(ctx context.Context, name string) (string, error)
	Skin(ctx context.Context, id string) ([]byte, error)
}

// BedrockSkins finds the skins of Bedrock players, which Geyser uploads to the textures server.
type BedrockSkins interface {
	SkinTexture(ctx context.Context, gamertag string) (string, error)
}

// Faces are the faces of players, cut from their skins, so that the browser never contacts
// Mojang or GeyserMC: of Java players from Mojang, of Bedrock players from GeyserMC. As names
// come from users and from agents, faces are kept, and only a few are looked up at a time.
type Faces struct {
	java    Skins
	bedrock BedrockSkins
	limit   *rate.Limiter
	lookups singleflight.Group

	mu    sync.Mutex
	known map[string]face // by name in lower case
}

type face struct {
	png []byte // nil without a face
	at  time.Time
}

func NewFaces(java Skins, bedrock BedrockSkins) *Faces {
	return &Faces{java: java, bedrock: bedrock, limit: rate.NewLimiter(rate.Every(lookupEvery), lookupBurst), known: map[string]face{}}
}

// Face returns the face of a player as a PNG image of 8 by 8 pixels, with the hat over it, or
// ErrNoFace.
func (f *Faces) Face(ctx context.Context, name string) ([]byte, error) {
	if !noryxv1.ValidPlayerName(name) {
		return nil, httpapi.Errorf(http.StatusBadRequest, "Enter the name of a player: up to 16 letters, digits and underscores.")
	}
	key := strings.ToLower(name)
	if known, ok := f.cached(key); ok {
		return known.found()
	}
	v, err, _ := f.lookups.Do(key, func() (any, error) {
		waitCtx, cancel := context.WithTimeout(ctx, lookupWait)
		defer cancel()
		if f.limit.Wait(waitCtx) != nil {
			return nil, errBusy
		}
		// The lookup isn't cut short by the request that began it, as others may wait for it.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), lookupTime)
		defer cancel()
		png, err := f.lookUp(ctx, name)
		if err != nil && !errors.Is(err, ErrNoFace) {
			return nil, err
		}
		known := face{png: png, at: time.Now()}
		f.keep(key, known)
		return known, nil
	})
	if err != nil {
		return nil, err
	}
	return v.(face).found()
}

func (k face) found() ([]byte, error) {
	if k.png == nil {
		return nil, ErrNoFace
	}
	return k.png, nil
}

func (f *Faces) cached(key string) (face, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	known, ok := f.known[key]
	if ok && time.Since(known.at) >= known.lifetime() {
		delete(f.known, key)
		return face{}, false
	}
	return known, ok
}

func (k face) lifetime() time.Duration {
	if k.png == nil {
		return noFaceTime
	}
	return faceTime
}

// keep keeps a face, and when maxFaces are kept, drops the expired ones, or else others.
func (f *Faces) keep(key string, known face) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.known) >= maxFaces {
		for k, v := range f.known {
			if time.Since(v.at) >= v.lifetime() || len(f.known) >= maxFaces {
				delete(f.known, k)
			}
		}
	}
	f.known[key] = known
}

// lookUp finds the skin of a player and cuts the face from it.
func (f *Faces) lookUp(ctx context.Context, name string) ([]byte, error) {
	var id string
	var err error
	if noryxv1.BedrockPlayer(name) {
		// Floodgate names Bedrock players by their gamertag, with underscores for spaces.
		id, err = f.bedrock.SkinTexture(ctx, strings.ReplaceAll(name[1:], "_", " "))
	} else {
		id, err = f.java.SkinTexture(ctx, name)
	}
	if err != nil {
		return nil, err
	}
	if id == "" {
		return nil, ErrNoFace
	}
	skin, err := f.java.Skin(ctx, id)
	if err != nil {
		return nil, err
	}
	return cutFace(skin)
}

// cutFace cuts the face of a skin of 64 by 64 or, as before Minecraft 1.8, 64 by 32 pixels,
// and puts the hat over it. Old skins have the hat opaque where they have none, so, as
// Minecraft does, it is left out of those unless it is transparent somewhere.
func cutFace(skin []byte) ([]byte, error) {
	cfg, err := png.DecodeConfig(bytes.NewReader(skin))
	if err != nil || cfg.Width != 64 || cfg.Height != 64 && cfg.Height != 32 {
		return nil, ErrNoFace
	}
	img, err := png.Decode(bytes.NewReader(skin))
	if err != nil {
		return nil, ErrNoFace
	}
	b := img.Bounds().Min
	out := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	draw.Draw(out, out.Bounds(), img, b.Add(image.Pt(8, 8)), draw.Src)
	hat := b.Add(image.Pt(40, 8))
	if cfg.Height == 64 || transparent(img, image.Rectangle{Min: hat, Max: hat.Add(image.Pt(8, 8))}) {
		draw.Draw(out, out.Bounds(), img, hat, draw.Over)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// transparent reports whether a part of an image is transparent somewhere.
func transparent(img image.Image, r image.Rectangle) bool {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0x8000 {
				return true
			}
		}
	}
	return false
}
