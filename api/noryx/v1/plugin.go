package noryxv1

import "regexp"

// pluginFolder matches the names plugins give themselves, which Bukkit limits to letters,
// digits, spaces, _, . and -, and the IDs of Velocity's plugins. A plugin keeps its settings in
// the folder of its name in the plugin folder, e.g. plugins/LuckPerms.
var pluginFolder = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9 _.-]{0,63}$`)

// ValidPluginFolder reports whether the name of a plugin can be the folder of its settings in
// the plugin folder, which the agent reads from the plugin's jar and the panel links to.
func ValidPluginFolder(name string) bool { return pluginFolder.MatchString(name) }
