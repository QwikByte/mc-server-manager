package docker

import (
	"cmp"
	"context"
	"encoding/json"
	"time"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/QwikByte/mc-server-manager/internal/agent/runtime"
)

func (d *Docker) Usage(ctx context.Context, id string) (runtime.Usage, error) {
	c, _, err := d.inspect(ctx, id)
	if err != nil {
		return runtime.Usage{}, err
	}
	if !c.State.Running {
		return runtime.Usage{}, runtime.ErrNotRunning
	}
	// A single sample returns right away; the caller computes rates from consecutive ones.
	res, err := d.cli.ContainerStats(ctx, containerName(id), client.ContainerStatsOptions{})
	if err != nil {
		return runtime.Usage{}, notFound(err)
	}
	defer res.Body.Close()
	var stats container.StatsResponse
	if err := json.NewDecoder(res.Body).Decode(&stats); err != nil {
		return runtime.Usage{}, err
	}
	mem := stats.MemoryStats
	// Like docker stats, without the page cache, which the kernel frees when it needs memory.
	cache := cmp.Or(mem.Stats["inactive_file"], mem.Stats["total_inactive_file"])
	u := runtime.Usage{
		CPUTime:     time.Duration(stats.CPUStats.CPUUsage.TotalUsage), //nolint:gosec // nanoseconds of CPU time fit easily
		MemoryBytes: mem.Usage - min(cache, mem.Usage),
		MemoryLimit: mem.Limit,
	}
	for _, n := range stats.Networks {
		u.NetRxBytes += n.RxBytes
		u.NetTxBytes += n.TxBytes
	}
	if ep := c.NetworkSettings.Networks[networkName]; ep != nil && ep.IPAddress.IsValid() {
		u.Host = ep.IPAddress.String()
	}
	return u, nil
}
