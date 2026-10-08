package player

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"sync/atomic"
	"testing"

	"golang.org/x/time/rate"

	"github.com/QwikByte/noryx/internal/master/httpapi"
)

var (
	red  = color.NRGBA{R: 255, A: 255}
	blue = color.NRGBA{B: 255, A: 255}
)

// skin draws a skin with a red face and a hat that is blue in its corner, and opaque white
// elsewhere unless transparent.
func skin(t *testing.T, height int, transparent bool) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 64, height))
	for y := 8; y < 16; y++ {
		for x := 8; x < 16; x++ {
			img.Set(x, y, red)
			if !transparent {
				img.Set(x+32, y, color.NRGBA{R: 255, G: 255, B: 255, A: 255})
			}
		}
	}
	img.Set(40, 8, blue)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func at(img image.Image, x, y int) color.Color { return color.NRGBAModel.Convert(img.At(x, y)) }

type fakeSkins struct {
	skins   map[string][]byte // by name, also the ID of their texture
	err     error
	lookups atomic.Int64
}

func (f *fakeSkins) SkinTexture(_ context.Context, name string) (string, error) {
	f.lookups.Add(1)
	if _, ok := f.skins[name]; !ok {
		return "", f.err
	}
	return name, f.err
}

func (f *fakeSkins) Skin(_ context.Context, id string) ([]byte, error) { return f.skins[id], nil }

func TestFaces(t *testing.T) {
	java := &fakeSkins{skins: map[string][]byte{"Alex": skin(t, 64, true), "Old": skin(t, 32, false), "Odd": skin(t, 16, true)}}
	bedrock := &fakeSkins{skins: map[string][]byte{"Tim 203": skin(t, 64, true)}}
	java.skins["Tim 203"] = bedrock.skins["Tim 203"]
	faces, ctx := NewFaces(java, bedrock), t.Context()
	face := func(name string) image.Image {
		t.Helper()
		data, err := faces.Face(ctx, name)
		if err != nil {
			t.Fatalf("face of %s: %v", name, err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil || img.Bounds().Dx() != 8 || img.Bounds().Dy() != 8 {
			t.Fatalf("face of %s: %v, %v", name, img.Bounds(), err)
		}
		return img
	}

	// The face has the hat over it; old skins have an opaque hat, which is left out.
	if img := face("Alex"); at(img, 0, 0) != blue || at(img, 1, 1) != red {
		t.Fatalf("face of Alex = %v, %v", img.At(0, 0), img.At(1, 1))
	}
	if img := face("Old"); at(img, 0, 0) != red {
		t.Fatalf("face of Old = %v", img.At(0, 0))
	}
	if img := face(".Tim_203"); at(img, 0, 0) != blue || bedrock.lookups.Load() != 1 {
		t.Fatalf("face of .Tim_203 = %v after %d lookups", img.At(0, 0), bedrock.lookups.Load())
	}

	// Faces and players without one are kept; names match regardless of case.
	face("alex")
	for _, name := range []string{"Nobody", "nobody", "Odd"} {
		if _, err := faces.Face(ctx, name); !errors.Is(err, ErrNoFace) {
			t.Errorf("face of %s: %v", name, err)
		}
	}
	if n := java.lookups.Load(); n != 4 {
		t.Errorf("%d lookups, want 4", n)
	}
	var invalid *httpapi.Error
	if _, err := faces.Face(ctx, "@a"); !errors.As(err, &invalid) || invalid.Status != 400 {
		t.Errorf("face of @a: %v", err)
	}

	// Failures aren't kept, and only so many faces are looked up at once.
	java.err = errors.New("unavailable")
	if _, err := faces.Face(ctx, "Steve"); err == nil || errors.Is(err, ErrNoFace) {
		t.Errorf("face while Mojang fails: %v", err)
	}
	java.err = nil
	faces.limit = rate.NewLimiter(0, 0)
	if _, err := faces.Face(ctx, "Steve"); !errors.Is(err, errBusy) {
		t.Errorf("face beyond the limit: %v", err)
	}
	if _, err := faces.Face(ctx, "Alex"); err != nil {
		t.Errorf("kept face beyond the limit: %v", err)
	}
}
