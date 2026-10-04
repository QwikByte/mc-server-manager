package docker

import (
	"cmp"
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	mcnet "github.com/QwikByte/mc-server-manager/internal/agent/network"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

// Configure writes the network configuration into the server's data directory. Game
// servers are recreated when they join or leave a network, because the online mode is part
// of the container's environment, and running ones restart to apply a changed
// configuration. Running proxies reload theirs.
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
	if spec.Type.Proxy() {
		return d.configureProxy(ctx, c, spec, data, network)
	}
	changed, err := mcnet.WriteBackend(data, spec.Type, network.Forwarding, network.ForwardingSecret)
	if err != nil {
		return err
	}
	running := c.State.Running
	if behind := network.Forwarding != runtime.ForwardingNone; behind != spec.BehindProxy || network.ProxyOnNode != spec.ProxyOnNode {
		spec.BehindProxy, spec.ProxyOnNode = behind, network.ProxyOnNode
		return d.recreate(ctx, spec, running, placement(c, spec))
	}
	if !running || !changed {
		return nil // applying the same configuration again must not kick players
	}
	return d.Restart(ctx, id)
}

// configureProxy writes a network into the configuration of a proxy, with the backends of
// this node in the proxy's Docker network, and makes a running proxy reload it.
func (d *Docker) configureProxy(ctx context.Context, c container.InspectResponse, spec runtime.Spec, data *datadir.Dir, network runtime.Network) error {
	if err := d.move(ctx, c, proxyNetwork(spec.ID)); err != nil { // proxies of older agents shared a network
		return err
	}
	local := network.Backends
	network.Backends = make([]runtime.NetworkBackend, len(local))
	for i, b := range local {
		address, err := d.address(ctx, spec.ID, b)
		if err != nil {
			return err
		}
		b.ServerID, b.Address = "", address
		network.Backends[i] = b
	}
	changed, removed, err := mcnet.WriteProxy(data, spec.Type, network)
	if err != nil {
		return err
	}
	if err := d.release(ctx, spec.ID, local); err != nil {
		return err
	}
	running := c.State.Running
	switch {
	case !mounted(c, images[spec.Type].data) || !c.Config.OpenStdin:
		// Created by an older agent that mounted the data and published the port wrongly,
		// or that didn't let the proxy read console commands.
		return d.recreate(ctx, spec, running, proxyNetwork(spec.ID))
	case !running || !changed:
		return nil
	case removed && spec.Type.Bungee():
		return d.Restart(ctx, spec.ID) // BungeeCord can't reload without a server it had
	}
	return d.Reload(ctx, spec.ID)
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
	sharedNetwork = "mcsm-servers"
	// legacyNetwork was shared by all servers of older agents, which could reach each other.
	legacyNetwork = "mcsm"
	// A Velocity proxy and its backends on the node have a network of their own.
	proxyNetworkPrefix = "mcsm-proxy-"
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
// agent, so that its name resolves to a single address. The network of older agents
// stays until the agent starts the server again, so that no player is disconnected now.
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

// prepare readies a server that is about to start, which disconnects its players anyway.
func (d *Docker) prepare(ctx context.Context, id string) error {
	c, spec, err := d.inspect(ctx, id)
	if err != nil {
		return err
	}
	if err := d.listen(spec); err != nil {
		return err
	}
	return d.leaveLegacy(ctx, c)
}

// listen makes a proxy listen on the port its container publishes, as the default
// configuration of the Velocity image listens on another one.
func (d *Docker) listen(spec runtime.Spec) error {
	if !spec.Type.Proxy() {
		return nil
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
	_, err = mcnet.ProxyBind(data, spec.Type, images[spec.Type].port)
	return err
}

// leaveLegacy takes a container out of the network of older agents, once it is in one
// of the current networks, and removes that network once empty.
func (d *Docker) leaveLegacy(ctx context.Context, c container.InspectResponse) error {
	if c.NetworkSettings == nil || c.NetworkSettings.Networks[legacyNetwork] == nil || len(c.NetworkSettings.Networks) < 2 {
		return nil
	}
	if _, err := d.cli.NetworkDisconnect(ctx, legacyNetwork, client.NetworkDisconnectOptions{Container: c.ID}); err != nil {
		return err
	}
	_, _ = d.cli.NetworkRemove(ctx, legacyNetwork, client.NetworkRemoveOptions{}) // fails while others use it
	return nil
}

// adopt moves the servers of older agents, which shared one network, into the current
// ones: a Velocity proxy and the backends its configuration names into the proxy's
// network, the others into the shared one. Stopped servers leave the old network now,
// running ones when the agent starts them again, so that no player is disconnected.
func (d *Docker) adopt(ctx context.Context) error {
	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", labelManaged).Add("network", legacyNetwork)})
	if err != nil {
		return err
	}
	var specs []runtime.Spec
	for _, c := range res.Items {
		if spec, ok := specOf(c.Labels); ok && slices.Contains(c.Names, "/"+containerName(spec.ID)) {
			specs = append(specs, spec)
		}
	}
	// Proxies first, which take their backends along.
	slices.SortFunc(specs, func(a, b runtime.Spec) int { return cmp.Compare(proxyRank(b), proxyRank(a)) })
	adopted := map[string]bool{}
	for _, spec := range specs {
		members := []string{spec.ID}
		to := sharedNetwork
		if spec.Type == mcsmv1.ServerType_SERVER_TYPE_VELOCITY {
			members, to = append(members, d.localBackends(spec)...), proxyNetwork(spec.ID)
		} else if adopted[spec.ID] {
			continue
		}
		for _, id := range members {
			c, _, err := d.inspect(ctx, id)
			if err != nil {
				return err
			}
			if err := d.move(ctx, c, to); err != nil {
				return err
			}
			adopted[id] = true
			if !c.State.Running {
				if c, _, err = d.inspect(ctx, id); err == nil {
					err = d.leaveLegacy(ctx, c)
				}
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func proxyRank(spec runtime.Spec) int {
	if spec.Type == mcsmv1.ServerType_SERVER_TYPE_VELOCITY {
		return 1
	}
	return 0
}

// localBackends returns the servers of this node that the configuration of a proxy names.
func (d *Docker) localBackends(proxy runtime.Spec) []string {
	path, err := d.dataPath(proxy)
	if err != nil {
		return nil
	}
	config, err := os.ReadFile(filepath.Join(path, "velocity.toml")) //nolint:gosec // a file of the proxy's data
	if err != nil {
		return nil
	}
	var ids []string
	for _, address := range mcnet.VelocityBackends(config) {
		host, _, _ := net.SplitHostPort(address)
		if id, ok := strings.CutPrefix(host, containerName("")); ok && runtime.ValidID(id) {
			ids = append(ids, id)
		}
	}
	return ids
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
