package network

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"go.yaml.in/yaml/v3"

	"github.com/QwikByte/mc-server-manager/internal/agent/datadir"
)

// format reads and writes a configuration file as nested maps; comments are not kept.
type format struct {
	unmarshal func([]byte, any) error
	marshal   func(any) ([]byte, error)
}

func formatOf(name string) format {
	if strings.HasSuffix(name, ".toml") {
		return format{toml.Unmarshal, toml.Marshal}
	}
	return format{yaml.Unmarshal, yaml.Marshal}
}

// parse reads the settings of a configuration file; an empty file has none.
func parse(name string, data []byte) (map[string]any, error) {
	settings := map[string]any{}
	if err := formatOf(name).unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("read %s: %w", name, err)
	}
	if settings == nil { // a YAML file of only comments
		settings = map[string]any{}
	}
	return settings, nil
}

// edit changes a configuration file of a server with fn and writes it only if that changed
// a setting, so that the file keeps its comments otherwise. It reports whether it wrote the
// file. A missing file is created, unless create is false.
func edit(dir *datadir.Dir, name string, create bool, fn func(settings map[string]any) error) (bool, error) {
	current, err := dir.ReadFile(name)
	exists := err == nil
	if err != nil && (!errors.Is(err, fs.ErrNotExist) || !create) {
		return false, ignoreMissing(err)
	}
	settings, err := parse(name, current)
	if err != nil {
		return false, err
	}
	f := formatOf(name)
	before, err := f.marshal(settings)
	if err != nil {
		return false, err
	}
	if err := fn(settings); err != nil {
		return false, err
	}
	after, err := f.marshal(settings)
	if err != nil || exists && bytes.Equal(before, after) {
		return false, err
	}
	if folder := path.Dir(name); folder != "." {
		if err := dir.MkdirAll(folder); err != nil {
			return false, err
		}
	}
	return true, dir.WriteFile(name, after)
}

func ignoreMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// child returns the map under key, which is created if it is missing.
func child(parent map[string]any, key string) map[string]any {
	m, ok := parent[key].(map[string]any)
	if !ok {
		m = map[string]any{}
		parent[key] = m
	}
	return m
}
