package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/QwikByte/noryx/internal/logging"
)

// tidyEvery is how often the references to what was deleted are removed, besides at the start
// and right after nodes, networks, datastores and notification channels are deleted in the
// panel.
const tidyEvery = time.Minute

// Tidy removes the references to nodes, networks, datastores, notification channels and
// workflows that no longer exist, whatever deleted them: from tasks such as backup jobs,
// workflows, notification rules, the users' alerts and file sets.
func (s Services) Tidy(ctx context.Context) {
	for _, p := range []interface{ Prune(context.Context) error }{s.Tasks, s.Workflows, s.Notify, s.Preferences, s.FileSets} {
		if err := p.Prune(ctx); err != nil && ctx.Err() == nil {
			slog.Warn("Can't remove the references to what was deleted", logging.System, "err", err)
		}
	}
}

// keepTidy calls tidy now and every tidyEvery until ctx is done.
func keepTidy(ctx context.Context, tidy func(context.Context)) {
	for {
		tidy(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(tidyEvery):
		}
	}
}
