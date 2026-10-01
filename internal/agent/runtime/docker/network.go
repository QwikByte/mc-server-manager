package docker

import (
	"context"
	"errors"
	"io/fs"
	"net"
	"path/filepath"
	"slices"
	"strconv"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	mcnet "github.com/QwikByte/mc-server-manager/internal/agent/network"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

// Configure writes the network configuration into the server's data directory. Game
// servers are recreated when they join or leave a network, because the online mode
// is part of the container's environment. Running servers restart to apply it.
func (d *Docker) Configure(ctx context.Context, id string, network runtime.Network) error {
	inspect, err := d.cli.ContainerInspect(ctx, containerName(id), client.ContainerInspectOptions{})
	if err != nil {
		return notFound(err)
	}
	spec, ok := specOf(inspect.Container.Config.Labels)
	if !ok {
		return runtime.ErrNotFound
	}
	running := inspect.Container.State.Running
	var changed bool

	switch spec.Type {
	case mcsmv1.ServerType_SERVER_TYPE_VELOCITY:
		if network.ForwardingSecret == "" {
			return errors.New("a proxy needs a forwarding secret")
		}
		if err := d.ensureNetwork(ctx); err != nil {
			return err
		}
		if err := d.join(ctx, inspect.Container); err != nil {
			return err
		}
		changed, err = d.writeProxyConfig(ctx, id, network)
		if err != nil {
			return err
		}
		if !mounted(inspect.Container, images[spec.Type].data) {
			// Created by an older agent that mounted the data and published the port wrongly.
			return d.recreate(ctx, spec, running)
		}
	case mcsmv1.ServerType_SERVER_TYPE_PAPER, mcsmv1.ServerType_SERVER_TYPE_PURPUR:
		changed, err = d.writeBackendConfig(id, network.ForwardingSecret)
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
	_, err = d.cli.ContainerRestart(ctx, containerName(id), client.ContainerRestartOptions{Timeout: new(stopTimeoutSeconds)})
	return err
}

// writeProxyConfig writes velocity.toml and the forwarding secret. It reports whether
// the network configuration changed.
func (d *Docker) writeProxyConfig(ctx context.Context, id string, network runtime.Network) (bool, error) {
	backends := make([]mcnet.Backend, 0, len(network.Backends))
	for _, b := range network.Backends {
		address, err := d.address(ctx, b)
		if err != nil {
			return false, err
		}
		backends = append(backends, mcnet.Backend{Name: b.Name, Address: address})
	}
	path := filepath.Join(id, "velocity.toml")
	current, err := d.readFile(path)
	if err != nil {
		return false, err
	}
	config, changed, err := mcnet.VelocityConfig(current, backends)
	if err != nil {
		return false, err
	}
	secretPath := filepath.Join(id, mcnet.ForwardingSecretFile)
	oldSecret, err := d.readFile(secretPath)
	if err != nil {
		return false, err
	}
	changed = changed || string(oldSecret) != network.ForwardingSecret
	if err := d.root.WriteFile(secretPath, []byte(network.ForwardingSecret), 0o600); err != nil {
		return false, err
	}
	return changed, d.root.WriteFile(path, config, 0o600)
}

// writeBackendConfig writes the forwarding settings of paper-global.yml. It reports
// whether they changed.
func (d *Docker) writeBackendConfig(id, secret string) (bool, error) {
	dir := filepath.Join(id, "config")
	current, err := d.readFile(filepath.Join(dir, "paper-global.yml"))
	if err != nil {
		return false, err
	}
	config, changed, err := mcnet.PaperGlobal(current, secret)
	if err != nil {
		return false, err
	}
	if err := d.root.MkdirAll(dir, 0o750); err != nil {
		return false, err
	}
	return changed, d.root.WriteFile(filepath.Join(dir, "paper-global.yml"), config, 0o600)
}

// readFile reads a file of the server data; a missing file reads as empty.
func (d *Docker) readFile(path string) ([]byte, error) {
	data, err := d.root.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// address returns where the proxy reaches a backend: by container name over the shared
// network for servers on this node, otherwise at the address given by the master.
func (d *Docker) address(ctx context.Context, b runtime.NetworkBackend) (string, error) {
	if b.ServerID == "" {
		return b.Address, nil
	}
	inspect, err := d.cli.ContainerInspect(ctx, containerName(b.ServerID), client.ContainerInspectOptions{})
	if err != nil {
		return "", notFound(err)
	}
	spec, ok := specOf(inspect.Container.Config.Labels)
	if !ok {
		return "", runtime.ErrNotFound
	}
	if err := d.join(ctx, inspect.Container); err != nil {
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

// recreate replaces the container of a server to change its environment. The data on
// the host is kept; a running server is stopped gracefully and started again.
func (d *Docker) recreate(ctx context.Context, spec runtime.Spec, running bool) error {
	if running {
		if err := d.Stop(ctx, spec.ID); err != nil {
			return err
		}
	}
	if _, err := d.cli.ContainerRemove(ctx, containerName(spec.ID), client.ContainerRemoveOptions{}); err != nil {
		return err
	}
	if err := d.createContainer(ctx, spec); err != nil {
		return err
	}
	if !running {
		return nil
	}
	return d.Start(ctx, spec.ID)
}
