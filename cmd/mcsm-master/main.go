// Command mcsm-master runs the admin panel and controls all node agents.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/QwikByte/mc-server-manager/internal/master/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := app.Command().ExecuteContext(ctx)
	var restart app.RestartExit
	switch {
	case errors.As(err, &restart):
		os.Exit(int(restart)) // its service manager starts the master again
	case err != nil:
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
