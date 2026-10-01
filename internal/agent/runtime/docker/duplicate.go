package docker

import (
	"context"
	"errors"
	"io/fs"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	mcnet "github.com/QwikByte/mc-server-manager/internal/agent/network"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

func (d *Docker) Duplicate(ctx context.Context, from string, spec runtime.Spec) (err error) {
	_, source, err := d.inspect(ctx, from)
	if err != nil {
		return err
	}
	spec.Type, spec.Storage, spec.BehindProxy = source.Type, source.Storage, false
	src, err := d.dataPath(source)
	if err != nil {
		return err
	}
	dst, err := d.dataPath(spec)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, d.removeData(spec))
		}
	}()
	if err := datadir.Copy(ctx, src, dst); err != nil {
		return err
	}
	if err := standalone(dst, source); err != nil {
		return err
	}
	// The image is present, as the container of the original uses it.
	return d.createContainer(ctx, spec)
}

// standalone resets the network role in the copy of a server's data. A backend stops
// trusting the proxy of the original, and a proxy loses the forwarding secret, which
// Velocity creates anew.
func standalone(path string, original runtime.Spec) error {
	data, err := datadir.Open(path)
	if err != nil {
		return err
	}
	defer data.Close()
	switch {
	case original.BehindProxy:
		_, err = writeBackendConfig(data, "")
	case original.Type == mcsmv1.ServerType_SERVER_TYPE_VELOCITY:
		if err = data.Remove(mcnet.ForwardingSecretFile); errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
	}
	return err
}
