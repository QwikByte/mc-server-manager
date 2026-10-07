package backup

import (
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	noryxv1 "github.com/QwikByte/noryx/api/noryx/v1"
	"github.com/QwikByte/noryx/internal/agent/datadir"
	"github.com/QwikByte/noryx/internal/agent/fileset"
	"github.com/QwikByte/noryx/internal/agent/plugin"
)

// notConfig are extensions of files in the data directory that aren't configuration: server
// jars, which the image downloads again, archives and logs.
var notConfig = []string{".jar", ".zip", ".gz", ".log"}

// selected returns the files and folders of a server's data that a selection covers, or "."
// for all of it, and those inside them that it leaves out. Data the server doesn't have, e.g.
// a missing custom path, is left out.
func selected(dir *datadir.Dir, typ noryxv1.ServerType, sel *noryxv1.BackupSelection) (paths, exclude []string, err error) {
	if len(sel.GetPaths()) > noryxv1.MaxBackupPaths || len(sel.GetExclude()) > noryxv1.MaxBackupPaths {
		return nil, nil, status.Errorf(codes.InvalidArgument, "Enter at most %d files or folders.", noryxv1.MaxBackupPaths)
	}
	for _, p := range sel.GetExclude() {
		name, ok := datadir.Name(p)
		if !ok || name == "." {
			return nil, nil, status.Errorf(codes.InvalidArgument, "The path %q is invalid.", p)
		}
		exclude = append(exclude, name)
	}
	if sel.GetEverything() {
		paths = append(paths, ".")
	}
	plugins, _ := plugin.Folder(typ)
	entries, err := fs.ReadDir(dir.FS(), ".")
	if err != nil {
		return nil, nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if datadir.IsTemp(name) || name == fileset.ManifestFile {
			continue
		}
		world := sel.GetWorlds() && e.IsDir() && isFile(dir, filepath.Join(name, "level.dat"))
		addons := sel.GetPlugins() && e.IsDir() && (name == plugins || plugins == "mods" && name == "config")
		config := sel.GetConfig() && ((e.IsDir() && name == "config") ||
			(e.Type().IsRegular() && !strings.HasPrefix(name, ".") && !slices.Contains(notConfig, strings.ToLower(filepath.Ext(name)))))
		if world || addons || config {
			paths = append(paths, name)
		}
	}
	for _, p := range sel.GetPaths() {
		name, ok := datadir.Name(p)
		if !ok {
			return nil, nil, status.Errorf(codes.InvalidArgument, "The path %q is invalid.", p)
		}
		if info, err := dir.Lstat(name); err == nil && name != fileset.ManifestFile && (info.IsDir() || info.Mode().IsRegular()) {
			paths = append(paths, name)
		}
	}
	// Only what is inside the paths matters; a path inside what is left out is left out.
	paths = slices.DeleteFunc(datadir.Outermost(paths), func(p string) bool { return slices.ContainsFunc(exclude, within(p)) })
	exclude = slices.DeleteFunc(datadir.Outermost(exclude), func(e string) bool { return !slices.ContainsFunc(paths, within(e)) })
	return paths, exclude, nil
}

// within returns whether a name is name, or inside it.
func within(name string) func(parent string) bool {
	return func(parent string) bool {
		return parent == "." || name == parent || strings.HasPrefix(name, parent+string(filepath.Separator))
	}
}

func isFile(dir *datadir.Dir, name string) bool {
	info, err := dir.Lstat(name)
	return err == nil && info.Mode().IsRegular()
}
