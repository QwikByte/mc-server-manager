package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"

	"github.com/QwikByte/noryx/internal/agent/progress"
	"github.com/QwikByte/noryx/internal/agent/runtime"
)

// upgradeFolder holds the dumps of an upgrade, in the folder of the datastore.
const upgradeFolder = "upgrade"

// upgrade moves a datastore to a newer major version, as the engines can't read the data of
// an older one, or not without the older one's programs: it dumps the databases with the
// hashes of their users' passwords, creates the container again on new data and loads them
// as their users. The old data stays as Previous, and a failure goes back to it. A stopped
// datastore starts for the time of the upgrade.
func (d *Docker) upgrade(ctx context.Context, c container.InspectResponse, current, spec runtime.DatastoreSpec, dir string) (err error) {
	eng := engines[spec.Engine]
	if slices.Index(eng.versions, spec.Version) <= slices.Index(eng.versions, current.Version) {
		return fmt.Errorf("%s %s can only move to a newer version of %s", spec.Engine.Slug(), current.Version, eng.versions)
	}
	if err := d.pullOrLocal(ctx, eng.image+":"+spec.Version); err != nil {
		return err
	}
	if !c.State.Running {
		defer func() {
			_, stopErr := d.cli.ContainerStop(context.WithoutCancel(ctx), datastoreName(spec.ID), client.ContainerStopOptions{Timeout: new(datastoreStop)})
			err = errors.Join(err, stopErr)
		}()
		if err := d.StartDatastore(ctx, spec.ID); err != nil {
			return err
		}
		if err := d.waitReady(ctx, spec.ID); err != nil {
			return err
		}
	}
	work := filepath.Join(dir, upgradeFolder)
	if err := os.RemoveAll(work); err != nil { // left by an upgrade the agent didn't finish
		return err
	}
	if err := os.Mkdir(work, 0o700); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(work)) }()
	names, credentials, err := d.dumpAll(ctx, spec.ID, work)
	if err != nil {
		return err
	}

	progress.Step(ctx, "container", 0)
	data := filepath.Join(dir, dataFolder(spec.Version))
	if err := os.RemoveAll(data); err != nil {
		return err
	}
	if err := newData(dir, spec.Version); err != nil {
		return err
	}
	next := spec
	next.Previous = current.Version
	if err := d.recreateDatastore(ctx, next, dir, true); err != nil {
		return errors.Join(err, os.RemoveAll(data))
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, d.recreateDatastore(context.WithoutCancel(ctx), current, dir, true), os.RemoveAll(data))
		}
	}()
	if err := d.waitReady(ctx, spec.ID); err != nil {
		return err
	}

	progress.Step(ctx, "load", 0)
	s, err := d.session(ctx, spec.ID)
	if err != nil {
		return err
	}
	for _, name := range names {
		if _, err := s.query(ctx, s.dialect.login(name, credentials[name], true)); err != nil {
			return err
		}
		if err := d.loadFile(ctx, spec.ID, name, filepath.Join(work, name+".sql")); err != nil {
			return fmt.Errorf("load %s: %w", name, err)
		}
	}
	if current.Previous != "" {
		return os.RemoveAll(filepath.Join(dir, dataFolder(current.Previous)))
	}
	return nil
}

// dumpAll dumps the databases of a datastore into work, as <name>.sql, and returns their
// names with the hashes of their users' passwords.
func (d *Docker) dumpAll(ctx context.Context, id, work string) ([]string, map[string]string, error) {
	progress.Step(ctx, "dump", 0)
	names, err := d.Databases(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	s, err := d.session(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	credentials := map[string]string{}
	for _, name := range names {
		if credentials[name], err = s.credential(ctx, name); err != nil {
			return nil, nil, err
		}
		f, err := os.OpenFile(filepath.Join(work, name+".sql"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) //nolint:gosec // a checked name in the folder of the datastore
		if err != nil {
			return nil, nil, err
		}
		if err := errors.Join(d.Dump(ctx, id, name, f), f.Close()); err != nil {
			return nil, nil, fmt.Errorf("dump %s: %w", name, err)
		}
	}
	return names, credentials, nil
}

func (d *Docker) loadFile(ctx context.Context, id, name, path string) error {
	f, err := os.Open(path) //nolint:gosec // written by dumpAll
	if err != nil {
		return err
	}
	defer f.Close()
	return d.Load(ctx, id, name, f)
}
