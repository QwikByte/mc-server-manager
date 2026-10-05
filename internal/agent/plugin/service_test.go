package plugin

import (
	"testing"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
)

func TestEveryServerTypeHasAFolder(t *testing.T) {
	for value, name := range noryxv1.ServerType_name {
		typ := noryxv1.ServerType(value)
		if _, ok := Folder(typ); !ok && typ != noryxv1.ServerType_SERVER_TYPE_UNSPECIFIED && typ != noryxv1.ServerType_SERVER_TYPE_VANILLA {
			t.Errorf("%s has no folder for plugins or mods", name)
		}
	}
}
