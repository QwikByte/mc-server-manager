package node

import (
	"bytes"
	"cmp"
	"context"
	"log/slog"
	"os"
	"os/exec"

	"golang.org/x/mod/semver"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	mcsmv1 "github.com/QwikByte/mc-server-manager/api/mcsm/v1"
	"github.com/QwikByte/mc-server-manager/internal/buildinfo"
	"github.com/QwikByte/mc-server-manager/internal/logging"
)

// installer is the installer that the agent's package ships.
const installer = "/usr/lib/mcsm-agent/install.sh"

// Update implements mcsmv1.NodeServiceServer. Only releases newer than the agent are
// installed, so even a compromised master can't downgrade a node to a vulnerable version.
// The installer runs in a unit of its own, which outlives the restart of the agent.
func (s *Service) Update(ctx context.Context, req *mcsmv1.UpdateRequest) (*mcsmv1.UpdateResponse, error) {
	v := req.GetVersion()
	if !buildinfo.IsRelease(v) || semver.Compare(v, buildinfo.Version) <= 0 {
		return nil, status.Errorf(codes.InvalidArgument, "%q is no release newer than the agent's version %s", v, buildinfo.Version)
	}
	if _, err := os.Stat(installer); err != nil {
		return nil, status.Error(codes.FailedPrecondition, "The agent wasn't installed from a package. Update it on the node with install.sh.")
	}
	//nolint:gosec // v is a release version, passed as an argument without a shell
	out, err := exec.CommandContext(ctx, "systemd-run", "--unit=mcsm-agent-update", "--collect", "--quiet",
		"--", installer, "update", "--only", "agent", "--version", v).CombinedOutput()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Can't start the update: %s", cmp.Or(string(bytes.TrimSpace(out)), err.Error()))
	}
	slog.Info("Update the node", logging.Nodes, "version", v)
	return &mcsmv1.UpdateResponse{}, nil
}
