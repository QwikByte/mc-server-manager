package docker

import (
	"context"
	"errors"
	"net"
	"slices"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	"github.com/QwikByte/noryx/internal/agent/datadir"
	mcnet "github.com/QwikByte/noryx/internal/agent/network"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// Configure writes the network configuration into the server's data directory. Game
// servers are recreated when they join or leave a network, or Bedrock players start or
// stop joining it, because the online mode and secure chat are part of the container's
// environment, and running ones restart to apply a changed configuration. Running proxies
// reload theirs, or restart if they must.
func (d *Docker) Configure(ctx context.Context, id string, network runtime.Network) (bool, error) {
	c, spec, err := d.inspect(ctx, id)
	if err != nil {
		return false, err
	}
	path, err := d.dataPath(spec)
	if err != nil {
		return false, err
	}
	data, err := datadir.Open(path)
	if err != nil {
		return false, err
	}
	defer data.Close()
	if spec.Type.Proxy() {
		return d.configureProxy(ctx, c, spec, data, network)
	}
	changed, err := mcnet.WriteBackend(data, spec.Type, network.Forwarding, network.ForwardingSecret)
	if err != nil {
		return false, err
	}
	running := c.State.Running
	if behind := network.Forwarding != runtime.ForwardingNone; behind != spec.BehindProxy || network.ProxyOnNode != spec.ProxyOnNode ||
		network.BedrockPlayers != spec.BedrockPlayers {
		if spec.BedrockPlayers && !network.BedrockPlayers {
			if err := mcnet.SecureChat(data); err != nil {
				return false, err
			}
		}
		spec.BehindProxy, spec.ProxyOnNode, spec.BedrockPlayers = behind, network.ProxyOnNode, network.BedrockPlayers
		return running, d.recreate(ctx, spec, running, placement(c, spec))
	}
	if !running || !changed {
		return false, nil // applying the same configuration again must not kick players
	}
	return true, d.Restart(ctx, id)
}

// configureProxy writes a network into the configuration of a proxy, with the backends of
// this node in the proxy's Docker network, and makes a running proxy reload it. A proxy
// whose Bedrock port changes is created again to publish it.
func (d *Docker) configureProxy(ctx context.Context, c container.InspectResponse, spec runtime.Spec, data *datadir.Dir, network runtime.Network) (bool, error) {
	if err := d.move(ctx, c, proxyNetwork(spec.ID)); err != nil { // proxies of older agents shared a network
		return false, err
	}
	local := network.Backends
	network.Backends = make([]runtime.NetworkBackend, len(local))
	for i, b := range local {
		address, err := d.address(ctx, spec.ID, b)
		if err != nil {
			return false, err
		}
		b.ServerID, b.Address = "", address
		network.Backends[i] = b
	}
	changed, removed, err := mcnet.WriteProxy(data, spec.Type, network)
	if err != nil {
		return false, err
	}
	geyser, err := mcnet.WriteGeyser(data, spec.Type, network.BedrockPort)
	if err != nil {
		return false, err
	}
	if err := d.release(ctx, spec.ID, local); err != nil {
		return false, err
	}
	running := c.State.Running
	switch {
	case network.BedrockPort != spec.BedrockPort || !mounted(c, images[spec.Type].data) || !c.Config.OpenStdin:
		// A new Bedrock port, or created by an older agent that mounted the data and
		// published the port wrongly, or that didn't let the proxy read console commands.
		spec.BedrockPort = network.BedrockPort
		return running, d.recreate(ctx, spec, running, proxyNetwork(spec.ID))
	case !running || !changed && !geyser:
		return false, nil
	case geyser || removed && spec.Type.Bungee():
		// Geyser reads its configuration when it starts, and BungeeCord can't reload without
		// a server it had.
		return true, d.Restart(ctx, spec.ID)
	}
	return false, d.Reload(ctx, spec.ID)
}

// address returns where a proxy reaches a backend: by container name over the network of
// the proxy for servers on this node, which joins it, otherwise at the address given by
// the master.
func (d *Docker) address(ctx context.Context, proxyID string, b runtime.NetworkBackend) (string, error) {
	if b.ServerID == "" {
		return b.Address, nil
	}
	c, spec, err := d.inspect(ctx, b.ServerID)
	if err != nil {
		return "", err
	}
	if err := d.move(ctx, c, proxyNetwork(proxyID)); err != nil {
		return "", err
	}
	return net.JoinHostPort(containerName(b.ServerID), strconv.Itoa(images[spec.Type].port)), nil
}

func mounted(c container.InspectResponse, dir string) bool {
	return slices.ContainsFunc(c.Mounts, func(m container.MountPoint) bool { return m.Destination == dir })
}

const (
	// sharedNetwork connects the servers of the node that aren't part of a network to the
	// internet, but not to each other.
	sharedNetwork = "noryx-servers"
	// A Velocity proxy and its backends on the node have a network of their own.
	proxyNetworkPrefix = "noryx-proxy-"
)

func proxyNetwork(proxyID string) string { return proxyNetworkPrefix + proxyID }

// home returns the network of a new server: a proxy gets its own, any other server the
// shared one. Backends move to the network of their proxy when it is configured.
func home(spec runtime.Spec) string {
	if spec.Type.Proxy() {
		return proxyNetwork(spec.ID)
	}
	return sharedNetwork
}

// placement returns the network of the new container of a server whose old one is c: a
// backend stays in the network of its proxy, any other server goes home.
func placement(c container.InspectResponse, spec runtime.Spec) string {
	if spec.BehindProxy && c.NetworkSettings != nil {
		for name := range c.NetworkSettings.Networks {
			if strings.HasPrefix(name, proxyNetworkPrefix) {
				return name
			}
		}
	}
	return home(spec)
}

// move puts a container into a network and takes it out of the other networks of the
// agent, so that its name resolves to a single address.
func (d *Docker) move(ctx context.Context, c container.InspectResponse, to string) error {
	if err := d.ensureNetwork(ctx, to); err != nil {
		return err
	}
	var attached map[string]*network.EndpointSettings
	if c.NetworkSettings != nil {
		attached = c.NetworkSettings.Networks
	}
	if attached[to] == nil {
		if _, err := d.cli.NetworkConnect(ctx, to, client.NetworkConnectOptions{Container: c.ID}); err != nil {
			return err
		}
	}
	for name := range attached {
		if name != to && (name == sharedNetwork || strings.HasPrefix(name, proxyNetworkPrefix)) {
			if _, err := d.cli.NetworkDisconnect(ctx, name, client.NetworkDisconnectOptions{Container: c.ID}); err != nil {
				return err
			}
		}
	}
	return nil
}

// release moves the servers that left the network of a proxy back to the shared network.
func (d *Docker) release(ctx context.Context, proxyID string, backends []runtime.NetworkBackend) error {
	res, err := d.cli.NetworkInspect(ctx, proxyNetwork(proxyID), client.NetworkInspectOptions{})
	if err != nil {
		return err
	}
	for containerID := range res.Network.Containers {
		c, err := d.cli.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
		if err != nil {
			return notFound(err)
		}
		spec, ok := specOf(c.Container.Config.Labels)
		if !ok || spec.ID == proxyID || slices.ContainsFunc(backends, func(b runtime.NetworkBackend) bool { return b.ServerID == spec.ID }) {
			continue
		}
		if err := d.move(ctx, c.Container, sharedNetwork); err != nil {
			return err
		}
	}
	return nil
}

// prepare readies a server that is about to start. A proxy gets its data handed to the
// user it runs as, and listens on the port its container publishes, as the default
// configuration of the Velocity image listens on another one. A stopped proxy of an older
// agent, which ran it as root, is created again to run as that user.
func (d *Docker) prepare(ctx context.Context, id string) error {
	c, spec, err := d.inspect(ctx, id)
	if err != nil || !spec.Type.Proxy() {
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
	if err := data.HandOver(proxyUID, proxyUID); err != nil {
		return err
	}
	if _, err := mcnet.ProxyBind(data, spec.Type, images[spec.Type].port); err != nil {
		return err
	}
	if c.Config.User != proxyUser && !c.State.Running {
		return d.recreate(ctx, spec, false, placement(c, spec))
	}
	return nil
}

// ensureNetwork creates a network of the agent if it doesn't exist yet. Containers in the
// shared network can't reach each other.
func (d *Docker) ensureNetwork(ctx context.Context, name string) error {
	_, err := d.cli.NetworkInspect(ctx, name, client.NetworkInspectOptions{})
	if !cerrdefs.IsNotFound(err) {
		return err
	}
	opts := client.NetworkCreateOptions{Driver: "bridge", Labels: map[string]string{labelManaged: "true"}}
	if name == sharedNetwork {
		opts.Options = map[string]string{"com.docker.network.bridge.enable_icc": "false"}
	}
	_, err = d.cli.NetworkCreate(ctx, name, opts)
	if cerrdefs.IsConflict(err) {
		return nil // created concurrently
	}
	return err
}

// recreate replaces the container of a server to change its settings, in network; the
// data on the host stays. The old container is kept until the new one exists, so that a failure
// leaves the server as it was. A running server is stopped gracefully and started again.
func (d *Docker) recreate(ctx context.Context, spec runtime.Spec, running bool, network string) error {
	if running {
		if err := d.Stop(ctx, spec.ID); err != nil {
			return err
		}
	}
	name, old := containerName(spec.ID), containerName(spec.ID)+"-old"
	if _, err := d.cli.ContainerRename(ctx, name, client.ContainerRenameOptions{NewName: old}); err != nil {
		return err
	}
	if err := d.createContainer(ctx, spec, network); err != nil {
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
