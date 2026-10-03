// Package docker runs Minecraft servers as Docker containers based on the
// itzg/minecraft-server and itzg/mc-proxy images, which cover all common server types.
package docker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"iter"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
	"github.com/QwikByte/mc-server-manager/internal/agent/storage"
)

const (
	labelManaged = "io.mcsm.managed"
	labelSpec    = "io.mcsm.spec"

	serverImage = "itzg/minecraft-server"
	proxyImage  = "itzg/mc-proxy"

	stopTimeoutSeconds = 60 // time for the server to save its worlds
	pidsLimit          = 1024
	maxLineBytes       = 1 << 20
)

// image describes how a server type maps onto the itzg images.
type image struct {
	ref  string // without tag
	typ  string // value of the TYPE variable
	port int    // port inside the container
	data string // data directory inside the container
}

// capabilities are all the images need: they start as root to hand the data directory to
// the server's user and switch to it. The proxy image works on as root in the directory
// it handed over, which needs reading it regardless of its permissions.
var capabilities = map[string][]string{
	serverImage: {"CHOWN", "SETUID", "SETGID"},
	proxyImage:  {"CHOWN", "SETUID", "SETGID", "DAC_READ_SEARCH"},
}

var images = map[mcsmv1.ServerType]image{
	mcsmv1.ServerType_SERVER_TYPE_VANILLA:    {serverImage, "VANILLA", 25565, "/data"},
	mcsmv1.ServerType_SERVER_TYPE_PAPER:      {serverImage, "PAPER", 25565, "/data"},
	mcsmv1.ServerType_SERVER_TYPE_PURPUR:     {serverImage, "PURPUR", 25565, "/data"},
	mcsmv1.ServerType_SERVER_TYPE_FABRIC:     {serverImage, "FABRIC", 25565, "/data"},
	mcsmv1.ServerType_SERVER_TYPE_FORGE:      {serverImage, "FORGE", 25565, "/data"},
	mcsmv1.ServerType_SERVER_TYPE_NEOFORGE:   {serverImage, "NEOFORGE", 25565, "/data"},
	mcsmv1.ServerType_SERVER_TYPE_VELOCITY:   {proxyImage, "VELOCITY", 25565, "/server"},
	mcsmv1.ServerType_SERVER_TYPE_BUNGEECORD: {proxyImage, "BUNGEECORD", 25577, "/server"},
}

// Docker implements runtime.Runtime. Container labels are the only state:
// the agent itself stores nothing besides the server data directories.
type Docker struct {
	cli     *client.Client
	storage *storage.Locations
}

// New connects to the Docker daemon configured by the standard DOCKER_* variables.
func New(locations *storage.Locations) (*Docker, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, err
	}
	return &Docker{cli: cli, storage: locations}, nil
}

func (d *Docker) Close() error { return d.cli.Close() }

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
		// The name check skips the old container while a server is being recreated.
		if spec, ok := specOf(c.Labels); ok && slices.Contains(c.Names, "/"+containerName(spec.ID)) {
			srv := runtime.Server{Spec: spec, State: state(c)}
			if mayHaveCrashed(c) {
				d.addCrashes(ctx, c.ID, &srv)
			}
			servers = append(servers, srv)
		}
	}
	return servers, nil
}

// specOf reads the server spec stored in the labels of a container created by the agent.
func specOf(labels map[string]string) (runtime.Spec, bool) {
	var spec runtime.Spec
	return spec, json.Unmarshal([]byte(labels[labelSpec]), &spec) == nil
}

func state(c container.Summary) mcsmv1.ServerState {
	switch {
	case c.State == container.StateRestarting: // after a crash, until Docker starts it again
		return mcsmv1.ServerState_SERVER_STATE_CRASHING
	case c.State != container.StateRunning:
		return mcsmv1.ServerState_SERVER_STATE_STOPPED
	case c.Health != nil && c.Health.Status == container.Starting:
		return mcsmv1.ServerState_SERVER_STATE_STARTING
	default:
		return mcsmv1.ServerState_SERVER_STATE_RUNNING
	}
}

func (d *Docker) Create(ctx context.Context, spec runtime.Spec) error {
	if _, ok := images[spec.Type]; !ok {
		return fmt.Errorf("server type %s is not supported by the docker runtime", spec.Type)
	}
	path, err := d.dataPath(spec)
	if err != nil {
		return err
	}
	if err := d.pull(ctx, imageRef(spec)); err != nil {
		return err
	}
	if err := os.Mkdir(path, 0o750); err != nil {
		return err
	}
	return d.createContainer(ctx, spec, home(spec))
}

// dataPath returns the host directory with the data of a server.
func (d *Docker) dataPath(spec runtime.Spec) (string, error) {
	location, err := d.storage.Path(spec.Storage)
	return filepath.Join(location, spec.ID), err
}

// inspect returns a container created by the agent and the spec of its server.
func (d *Docker) inspect(ctx context.Context, id string) (container.InspectResponse, runtime.Spec, error) {
	res, err := d.cli.ContainerInspect(ctx, containerName(id), client.ContainerInspectOptions{})
	if err != nil {
		return container.InspectResponse{}, runtime.Spec{}, notFound(err)
	}
	spec, ok := specOf(res.Container.Config.Labels)
	if !ok || spec.ID != id {
		return container.InspectResponse{}, runtime.Spec{}, runtime.ErrNotFound
	}
	return res.Container, spec, nil
}

func (d *Docker) Data(ctx context.Context, id string) (*datadir.Dir, error) {
	_, spec, err := d.inspect(ctx, id)
	if err != nil {
		return nil, err
	}
	path, err := d.dataPath(spec)
	if err != nil {
		return nil, err
	}
	return datadir.Open(path)
}

// imageRef returns the image of a server. The tags of the server image select the
// Java version; latest has the newest.
func imageRef(spec runtime.Spec) string {
	img := images[spec.Type]
	if spec.Java != "" && img.ref == serverImage {
		return img.ref + ":java" + spec.Java
	}
	return img.ref + ":latest"
}

var restartPolicies = map[mcsmv1.RestartPolicy]container.RestartPolicyMode{
	mcsmv1.RestartPolicy_RESTART_POLICY_UNSPECIFIED: container.RestartPolicyUnlessStopped,
	mcsmv1.RestartPolicy_RESTART_POLICY_ALWAYS:      container.RestartPolicyUnlessStopped,
	mcsmv1.RestartPolicy_RESTART_POLICY_ON_CRASH:    container.RestartPolicyOnFailure,
	mcsmv1.RestartPolicy_RESTART_POLICY_NEVER:       container.RestartPolicyDisabled,
}

// createContainer creates the container of a server whose image and data directory exist,
// in the network netName.
func (d *Docker) createContainer(ctx context.Context, spec runtime.Spec, netName string) error {
	path, err := d.dataPath(spec)
	if err != nil {
		return err
	}
	if err := d.ensureNetwork(ctx, netName); err != nil {
		return err
	}
	img := images[spec.Type]
	specJSON, err := json.Marshal(spec)
	if err != nil {
		return err
	}
	// EULA=TRUE is only set because the operator accepted the EULA when creating the server.
	env := []string{"EULA=TRUE", "TYPE=" + img.typ, "VERSION=" + spec.Version, fmt.Sprintf("MEMORY=%dM", spec.MemoryMB)}
	if spec.BehindProxy {
		env = append(env, "ONLINE_MODE=FALSE") // the proxy authenticates players
	}
	if spec.AikarFlags {
		env = append(env, "USE_AIKAR_FLAGS=TRUE")
	}
	if len(spec.JVMOptions) > 0 {
		env = append(env, "JVM_OPTS="+strings.Join(spec.JVMOptions, " "))
	}
	port := network.MustParsePort(fmt.Sprintf("%d/tcp", img.port))
	published := network.PortMap{port: {{HostPort: strconv.Itoa(int(spec.Port))}}}
	if spec.BehindProxy && spec.ProxyOnNode {
		published = nil // only its proxy connects, over their network
	}
	_, err = d.cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: containerName(spec.ID),
		Config: &container.Config{
			Image:        imageRef(spec),
			Env:          env,
			Labels:       map[string]string{labelManaged: "true", labelSpec: string(specJSON)},
			ExposedPorts: network.PortSet{port: {}},
		},
		NetworkingConfig: &network.NetworkingConfig{EndpointsConfig: map[string]*network.EndpointSettings{netName: {}}},
		HostConfig: &container.HostConfig{
			Binds:         []string{path + ":" + img.data},
			PortBindings:  published,
			RestartPolicy: container.RestartPolicy{Name: restartPolicies[spec.RestartPolicy]},
			SecurityOpt:   []string{"no-new-privileges:true"},
			CapDrop:       []string{"ALL"},
			CapAdd:        capabilities[img.ref],
			Resources: container.Resources{
				// The JVM needs memory beyond its heap, so the hard limit gets some headroom.
				Memory:    int64(spec.MemoryMB*5/4+256) << 20,
				NanoCPUs:  int64(spec.CPUMillis) * 1e6,
				PidsLimit: new(int64(pidsLimit)),
			},
		},
	})
	return err
}

func (d *Docker) Start(ctx context.Context, id string) error {
	if err := d.prepare(ctx, id); err != nil {
		return err
	}
	_, err := d.cli.ContainerStart(ctx, containerName(id), client.ContainerStartOptions{})
	return notFound(err)
}

func (d *Docker) Stop(ctx context.Context, id string) error {
	_, err := d.cli.ContainerStop(ctx, containerName(id), client.ContainerStopOptions{Timeout: new(stopTimeoutSeconds)})
	return notFound(err)
}

func (d *Docker) Update(ctx context.Context, spec runtime.Spec) error {
	c, current, err := d.inspect(ctx, spec.ID)
	if err != nil {
		return err
	}
	spec.Type, spec.Storage, spec.BehindProxy, spec.ProxyOnNode = current.Type, current.Storage, current.BehindProxy, current.ProxyOnNode
	if imageRef(spec) != imageRef(current) {
		if err := d.pull(ctx, imageRef(spec)); err != nil {
			return err
		}
	}
	return d.recreate(ctx, spec, c.State.Running, placement(c, spec))
}

// Restart stops a server gracefully and starts it again.
func (d *Docker) Restart(ctx context.Context, id string) error {
	if err := d.prepare(ctx, id); err != nil {
		return err
	}
	_, err := d.cli.ContainerRestart(ctx, containerName(id), client.ContainerRestartOptions{Timeout: new(stopTimeoutSeconds)})
	return notFound(err)
}

// Remove deletes the container and all server data.
func (d *Docker) Remove(ctx context.Context, id string) error {
	_, spec, err := d.inspect(ctx, id)
	if err != nil {
		return err
	}
	if _, err := d.dataPath(spec); err != nil { // e.g. a removed storage location; nothing is deleted yet
		return err
	}
	if _, err := d.cli.ContainerRemove(ctx, containerName(id), client.ContainerRemoveOptions{Force: true}); err != nil {
		return notFound(err)
	}
	if spec.Type == mcsmv1.ServerType_SERVER_TYPE_VELOCITY {
		if _, err := d.cli.NetworkRemove(ctx, proxyNetwork(id), client.NetworkRemoveOptions{}); err != nil && !cerrdefs.IsNotFound(err) {
			return err
		}
	}
	return d.removeData(spec)
}

// removeData deletes the data directory of a server.
func (d *Docker) removeData(spec runtime.Spec) error {
	location, err := d.storage.Path(spec.Storage)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(location)
	if err != nil {
		return err
	}
	return errors.Join(root.RemoveAll(spec.ID), root.Close())
}

func (d *Docker) Logs(ctx context.Context, id string, tail int, after time.Time) iter.Seq2[runtime.LogLine, error] {
	return func(yield func(runtime.LogLine, error) bool) {
		opts := client.ContainerLogsOptions{ShowStdout: true, ShowStderr: true, Follow: true, Timestamps: true, Tail: strconv.Itoa(tail)}
		if !after.IsZero() {
			opts.Since = after.Format(time.RFC3339Nano) // includes the line written at that time
		}
		logs, err := d.cli.ContainerLogs(ctx, containerName(id), opts)
		if err != nil {
			yield(runtime.LogLine{}, notFound(err))
			return
		}
		defer logs.Close()
		// Docker multiplexes stdout and stderr into one stream; both end up in the console.
		r, w := io.Pipe()
		defer r.Close()
		go func() {
			_, err := stdcopy.StdCopy(w, w, logs)
			w.CloseWithError(err)
		}()
		lines := bufio.NewScanner(r)
		lines.Buffer(make([]byte, 0, 64<<10), maxLineBytes)
		for lines.Scan() {
			line := logLine(lines.Text())
			if (after.IsZero() || line.Time.After(after)) && !yield(line, nil) {
				return
			}
		}
		if err := lines.Err(); err != nil && ctx.Err() == nil {
			yield(runtime.LogLine{}, err)
		}
	}
}

// logLine splits off the time Docker writes in front of every line.
func logLine(text string) runtime.LogLine {
	stamp, rest, _ := strings.Cut(text, " ")
	t, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		return runtime.LogLine{Text: text}
	}
	return runtime.LogLine{Time: t, Text: rest}
}

// SendCommand runs the command through rcon-cli, which the itzg server image ships
// together with a preconfigured RCON connection. Proxies have no RCON.
func (d *Docker) SendCommand(ctx context.Context, id, command string) (string, error) {
	c, spec, err := d.inspect(ctx, id)
	if err != nil {
		return "", err
	}
	if images[spec.Type].ref != serverImage {
		return "", runtime.ErrUnsupported
	}
	if !c.State.Running {
		return "", runtime.ErrNotRunning
	}
	// As the server's user: root in the container may not read its data.
	user, err := d.owner(spec)
	if err != nil {
		return "", err
	}
	exec, err := d.cli.ExecCreate(ctx, containerName(id), client.ExecCreateOptions{
		User: user, Cmd: []string{"rcon-cli", command}, AttachStdout: true, AttachStderr: true,
	})
	if err != nil {
		return "", err
	}
	attached, err := d.cli.ExecAttach(ctx, exec.ID, client.ExecAttachOptions{})
	if err != nil {
		return "", err
	}
	defer attached.Close()
	var out bytes.Buffer
	if _, err := stdcopy.StdCopy(&out, &out, io.LimitReader(attached.Reader, maxLineBytes)); err != nil {
		return "", err
	}
	result, err := d.cli.ExecInspect(ctx, exec.ID, client.ExecInspectOptions{})
	if err != nil {
		return "", err
	}
	if result.ExitCode != 0 {
		return "", fmt.Errorf("rcon-cli failed: %s", strings.TrimSpace(out.String()))
	}
	return out.String(), nil
}

func (d *Docker) pull(ctx context.Context, ref string) error {
	res, err := d.cli.ImagePull(ctx, ref, client.ImagePullOptions{})
	if err == nil {
		defer res.Close()
		err = res.Wait(ctx)
	}
	if err != nil {
		return fmt.Errorf("pull %s: %w", ref, err)
	}
	return nil
}

// owner returns the user and group that own the data of a server, as uid:gid; the image
// hands the data to the user it runs the server as.
func (d *Docker) owner(spec runtime.Spec) (string, error) {
	path, err := d.dataPath(spec)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("the owner of the server's data is unknown")
	}
	return fmt.Sprintf("%d:%d", st.Uid, st.Gid), nil
}

func containerName(id string) string { return "mcsm-" + id }

func notFound(err error) error {
	if cerrdefs.IsNotFound(err) {
		return runtime.ErrNotFound
	}
	return err
}
