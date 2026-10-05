package docker

import (
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func TestEveryServerTypeHasAnImage(t *testing.T) {
	for value, name := range noryxv1.ServerType_name {
		if typ := noryxv1.ServerType(value); typ != noryxv1.ServerType_SERVER_TYPE_UNSPECIFIED && images[typ].ref == "" {
			t.Errorf("%s has no image", name)
		}
	}
}
