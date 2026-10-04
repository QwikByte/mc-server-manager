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
	"github.com/QwikByte/noryx/internal/agent/plugin"
)

// notConfig are extensions of files in the data directory that aren't configuration: server
// jars, which the image downloads again, archives and logs.
var notConfig = []string{".jar", ".zip", ".gz", ".log"}

// selected returns the files and folders of a server's data that a selection covers, or "."
// for all of it. Data the server doesn't have, e.g. a missing custom path, is left out.
func selected(dir *datadir.Dir, typ noryxv1.ServerType, sel *noryxv1.BackupSelection) ([]string, error) {
	var paths []string
	if sel.GetEverything() {
		paths = append(paths, ".")
	}
	plugins, _ := plugin.Folder(typ)
	entries, err := fs.ReadDir(dir.FS(), ".")
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		name := e.Name()
		if datadir.IsTemp(name) {
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
			return nil, status.Errorf(codes.InvalidArgument, "The path %q is invalid.", p)
		}
		if info, err := dir.Lstat(name); err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			paths = append(paths, name)
		}
	}
	return outermost(paths), nil
}

func isFile(dir *datadir.Dir, name string) bool {
	info, err := dir.Lstat(name)
	return err == nil && info.Mode().IsRegular()
}

// outermost removes duplicates and paths inside others from a list of paths.
func outermost(paths []string) []string {
	if slices.Contains(paths, ".") {
		return []string{"."}
	}
	slices.Sort(paths)
	all := slices.Compact(slices.Clone(paths))
	return slices.DeleteFunc(slices.Compact(paths), func(p string) bool {
		return slices.ContainsFunc(all, func(parent string) bool { return strings.HasPrefix(p, parent+string(filepath.Separator)) })
	})
}
