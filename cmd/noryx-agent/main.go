// Command noryx-agent runs on every node and manages its Minecraft servers.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/QwikByte/mc-server-manager/internal/agent/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Command().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
