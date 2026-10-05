package agentcli

import (
	"context"
	"io"
	"testing"

	"google.golang.org/grpc"
)

func TestLogsRefusesNegativeLines(t *testing.T) {
	cmd := cli{func(context.Context, func(grpc.ClientConnInterface) error) error {
		t.Fatal("connected to the agent")
		return nil
	}}.logs()
	cmd.SetArgs([]string{"-n", "-5"})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err == nil || err.Error() != "--lines can't be negative" {
		t.Fatalf("err = %v", err)
	}
}
