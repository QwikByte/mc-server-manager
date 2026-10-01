// Command mcsm-master runs the admin panel and controls all node agents.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/QwikByte/mc-server-manager/internal/master/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := app.Command().ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}
