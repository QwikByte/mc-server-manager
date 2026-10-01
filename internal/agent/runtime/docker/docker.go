// Package docker runs Minecraft servers as Docker containers based on the
// itzg/minecraft-server and itzg/mc-proxy images, which cover all common server types.
package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

const (
	labelManaged = "io.mcsm.managed"
	labelSpec    = "io.mcsm.spec"

	serverImage = "itzg/minecraft-server:latest"
	proxyImage  = "itzg/mc-proxy:latest"

	stopTimeoutSeconds = 60 // time for the server to save its worlds
	pidsLimit          = 1024
)

// image describes how a server type maps onto the itzg images.
type image struct {
	ref  string
	typ  string // value of the TYPE variable
	port int    // port inside the container
}

var images = map[mcsmv1.ServerType]image{
	mcsmv1.ServerType_SERVER_TYPE_VANILLA:    {serverImage, "VANILLA", 25565},
	mcsmv1.ServerType_SERVER_TYPE_PAPER:      {serverImage, "PAPER", 25565},
	mcsmv1.ServerType_SERVER_TYPE_PURPUR:     {serverImage, "PURPUR", 25565},
	mcsmv1.ServerType_SERVER_TYPE_FABRIC:     {serverImage, "FABRIC", 25565},
	mcsmv1.ServerType_SERVER_TYPE_FORGE:      {serverImage, "FORGE", 25565},
	mcsmv1.ServerType_SERVER_TYPE_NEOFORGE:   {serverImage, "NEOFORGE", 25565},
	mcsmv1.ServerType_SERVER_TYPE_VELOCITY:   {proxyImage, "VELOCITY", 25577},
	mcsmv1.ServerType_SERVER_TYPE_BUNGEECORD: {proxyImage, "BUNGEECORD", 25577},
}

// Docker implements runtime.Runtime. Container labels are the only state:
// the agent itself stores nothing besides the server data directories.
type Docker struct {
	cli     *client.Client
	dataDir string
	root    *os.Root // confines file operations to dataDir
}

// New connects to the Docker daemon configured by the standard DOCKER_* variables.
func New(dataDir string) (*Docker, error) {
	if err := os.MkdirAll(dataDir, 0o750); err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dataDir)
	if err != nil {
		return nil, err
	}
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, errors.Join(err, root.Close())
	}
	return &Docker{cli: cli, dataDir: dataDir, root: root}, nil
}

func (d *Docker) Close() error { return errors.Join(d.cli.Close(), d.root.Close()) }

func (d *Docker) Info(ctx context.Context) (runtime.Info, error) {
	res, err := d.cli.Info(ctx, client.InfoOptions{})
	if err != nil {
		return runtime.Info{}, err
	}
	i := res.Info
	//nolint:gosec // CPU count and memory size are never negative and fit easily
	return runtime.Info{Name: "docker " + i.ServerVersion, OS: i.OperatingSystem, CPUs: uint32(i.NCPU), MemoryBytes: uint64(i.MemTotal)}, nil
}

func (d *Docker) List(ctx context.Context) ([]runtime.Server, error) {
	res, err := d.cli.ContainerList(ctx, client.ContainerListOptions{All: true, Filters: make(client.Filters).Add("label", labelManaged)})
	if err != nil {
		return nil, err
	}
	servers := make([]runtime.Server, 0, len(res.Items))
	for _, c := range res.Items {
		var spec runtime.Spec
		if err := json.Unmarshal([]byte(c.Labels[labelSpec]), &spec); err != nil {
			continue // not created by a compatible agent
		}
		servers = append(servers, runtime.Server{Spec: spec, State: state(c)})
	}
	return servers, nil
}

func state(c container.Summary) mcsmv1.ServerState {
	switch {
	case c.State != container.StateRunning && c.State != container.StateRestarting:
		return mcsmv1.ServerState_SERVER_STATE_STOPPED
	case c.State == container.StateRestarting, c.Health != nil && c.Health.Status == container.Starting:
		return mcsmv1.ServerState_SERVER_STATE_STARTING
	default:
		return mcsmv1.ServerState_SERVER_STATE_RUNNING
	}
}

func (d *Docker) Create(ctx context.Context, spec runtime.Spec) error {
	img, ok := images[spec.Type]
	if !ok {
		return fmt.Errorf("server type %s is not supported by the docker runtime", spec.Type)
	}
	if err := d.pull(ctx, img.ref); err != nil {
		return fmt.Errorf("pull %s: %w", img.ref, err)
	}
	if err := d.root.MkdirAll(spec.ID, 0o750); err != nil {
		return err
	}
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	port := network.MustParsePort(fmt.Sprintf("%d/tcp", img.port))
	_, err = d.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: containerName(spec.ID),
		Config: &container.Config{
			Image: img.ref,
			// EULA=TRUE is only set because the operator accepted the EULA when creating the server.
			Env:          []string{"EULA=TRUE", "TYPE=" + img.typ, "VERSION=" + spec.Version, fmt.Sprintf("MEMORY=%dM", spec.MemoryMB)},
			Labels:       map[string]string{labelManaged: "true", labelSpec: string(specJSON)},
			ExposedPorts: network.PortSet{port: {}},
		},
		HostConfig: &container.HostConfig{
			Binds:         []string{filepath.Join(d.dataDir, spec.ID) + ":/data"},
			PortBindings:  network.PortMap{port: {{HostPort: strconv.Itoa(int(spec.Port))}}},
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyUnlessStopped},
			SecurityOpt:   []string{"no-new-privileges:true"},
			Resources: container.Resources{
				// The JVM needs memory beyond its heap, so the hard limit gets some headroom.
				Memory:    int64(spec.MemoryMB*5/4+256) << 20,
				PidsLimit: new(int64(pidsLimit)),
			},
		},
	})
	return err
}

func (d *Docker) Start(ctx context.Context, id string) error {
	_, err := d.cli.ContainerStart(ctx, containerName(id), client.ContainerStartOptions{})
	return notFound(err)
}

func (d *Docker) Stop(ctx context.Context, id string) error {
	_, err := d.cli.ContainerStop(ctx, containerName(id), client.ContainerStopOptions{Timeout: new(stopTimeoutSeconds)})
	return notFound(err)
}

// Remove deletes the container and all server data.
func (d *Docker) Remove(ctx context.Context, id string) error {
	if _, err := d.cli.ContainerRemove(ctx, containerName(id), client.ContainerRemoveOptions{Force: true}); err != nil {
		return notFound(err)
	}
	return d.root.RemoveAll(id)
}

func (d *Docker) pull(ctx context.Context, ref string) error {
	res, err := d.cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer res.Close()
	return res.Wait(ctx)
}

func containerName(id string) string { return "mcsm-" + id }

func notFound(err error) error {
	if cerrdefs.IsNotFound(err) {
		return runtime.ErrNotFound
	}
	return err
}
