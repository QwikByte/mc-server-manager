package docker

import (
	"context"
	"errors"
	"net"
	"slices"
	"strconv"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	mcnet "github.com/QwikByte/mc-server-manager/internal/agent/network"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

// Configure writes the network configuration into the server's data directory. Game
// servers are recreated when they join or leave a network, because the online mode
// is part of the container's environment. Running servers restart to apply it.
func (d *Docker) Configure(ctx context.Context, id string, network runtime.Network) error {
	c, spec, err := d.inspect(ctx, id)
	if err != nil {
		return err
	}
	path, err := d.dataPath(spec)
	if err != nil {
		return err
	}
	data, err := datadir.Open(path)
	if err != nil {
		return err
	}
	defer data.Close()
	running := c.State.Running
	var changed bool

	switch spec.Type {
	case mcsmv1.ServerType_SERVER_TYPE_VELOCITY:
		if network.ForwardingSecret == "" {
			return errors.New("a proxy needs a forwarding secret")
		}
		if err := d.ensureNetwork(ctx); err != nil {
			return err
		}
		if err := d.join(ctx, c); err != nil {
			return err
		}
		changed, err = d.writeProxyConfig(ctx, data, network)
		if err != nil {
			return err
		}
		if !mounted(c, images[spec.Type].data) {
			// Created by an older agent that mounted the data and published the port wrongly.
			return d.recreate(ctx, spec, running)
		}
	case mcsmv1.ServerType_SERVER_TYPE_PAPER, mcsmv1.ServerType_SERVER_TYPE_PURPUR:
		changed, err = writeBackendConfig(data, network.ForwardingSecret)
		if err != nil {
			return err
		}
		if behind := network.ForwardingSecret != ""; behind != spec.BehindProxy {
			spec.BehindProxy = behind
			return d.recreate(ctx, spec, running)
		}
	default:
		return runtime.ErrUnsupported
	}
	if !running || !changed {
		return nil // applying the same configuration again must not kick players
	}
	return d.Restart(ctx, id)
}

// writeProxyConfig writes velocity.toml and the forwarding secret. It reports whether
// the network configuration changed.
func (d *Docker) writeProxyConfig(ctx context.Context, data *datadir.Dir, network runtime.Network) (bool, error) {
	backends := make([]mcnet.Backend, 0, len(network.Backends))
	for _, b := range network.Backends {
		address, err := d.address(ctx, b)
		if err != nil {
			return false, err
		}
		backends = append(backends, mcnet.Backend{Name: b.Name, Address: address})
	}
	current, err := data.ReadOptional("velocity.toml")
	if err != nil {
		return false, err
	}
	config, changed, err := mcnet.VelocityConfig(current, backends)
	if err != nil {
		return false, err
	}
	oldSecret, err := data.ReadOptional(mcnet.ForwardingSecretFile)
	if err != nil {
		return false, err
	}
	changed = changed || string(oldSecret) != network.ForwardingSecret
	if err := data.WriteFile(mcnet.ForwardingSecretFile, []byte(network.ForwardingSecret)); err != nil {
		return false, err
	}
	return changed, data.WriteFile("velocity.toml", config)
}

// writeBackendConfig writes the forwarding settings of paper-global.yml. It reports
// whether they changed.
func writeBackendConfig(data *datadir.Dir, secret string) (bool, error) {
	const file = "config/paper-global.yml"
	current, err := data.ReadOptional(file)
	if err != nil {
		return false, err
	}
	config, changed, err := mcnet.PaperGlobal(current, secret)
	if err != nil {
		return false, err
	}
	if err := data.MkdirAll("config"); err != nil {
		return false, err
	}
	return changed, data.WriteFile(file, config)
}

// address returns where the proxy reaches a backend: by container name over the shared
// network for servers on this node, otherwise at the address given by the master.
func (d *Docker) address(ctx context.Context, b runtime.NetworkBackend) (string, error) {
	if b.ServerID == "" {
		return b.Address, nil
	}
	c, spec, err := d.inspect(ctx, b.ServerID)
	if err != nil {
		return "", err
	}
	if err := d.join(ctx, c); err != nil {
		return "", err
	}
	return net.JoinHostPort(containerName(b.ServerID), strconv.Itoa(images[spec.Type].port)), nil
}

func mounted(c container.InspectResponse, dir string) bool {
	return slices.ContainsFunc(c.Mounts, func(m container.MountPoint) bool { return m.Destination == dir })
}

// join connects a container to the shared network; containers of older agent versions
// are not attached to it yet.
func (d *Docker) join(ctx context.Context, c container.InspectResponse) error {
	if c.NetworkSettings != nil && c.NetworkSettings.Networks[networkName] != nil {
		return nil
	}
	_, err := d.cli.NetworkConnect(ctx, networkName, client.NetworkConnectOptions{Container: c.ID})
	return err
}

func (d *Docker) ensureNetwork(ctx context.Context) error {
	_, err := d.cli.NetworkInspect(ctx, networkName, client.NetworkInspectOptions{})
	if !cerrdefs.IsNotFound(err) {
		return err
	}
	_, err = d.cli.NetworkCreate(ctx, networkName, client.NetworkCreateOptions{Driver: "bridge", Labels: map[string]string{labelManaged: "true"}})
	if cerrdefs.IsConflict(err) {
		return nil // created concurrently
	}
	return err
}

// recreate replaces the container of a server to change its settings; the data on the
// host stays. The old container is kept until the new one exists, so that a failure
// leaves the server as it was. A running server is stopped gracefully and started again.
func (d *Docker) recreate(ctx context.Context, spec runtime.Spec, running bool) error {
	if running {
		if err := d.Stop(ctx, spec.ID); err != nil {
			return err
		}
	}
	name, old := containerName(spec.ID), containerName(spec.ID)+"-old"
	if _, err := d.cli.ContainerRename(ctx, name, client.ContainerRenameOptions{NewName: old}); err != nil {
		return err
	}
	if err := d.createContainer(ctx, spec); err != nil {
		_, renameErr := d.cli.ContainerRename(ctx, old, client.ContainerRenameOptions{NewName: name})
		if renameErr == nil && running {
			renameErr = d.Start(ctx, spec.ID)
		}
		return errors.Join(err, renameErr)
	}
	if _, err := d.cli.ContainerRemove(ctx, old, client.ContainerRemoveOptions{}); err != nil {
		return err
	}
	if !running {
		return nil
	}
	return d.Start(ctx, spec.ID)
}
