package noryxv1

// ConfigFile returns the configuration file in the data of a proxy: velocity.toml of
// Velocity, config.yml of BungeeCord and Waterfall.
func (t ServerType) ConfigFile() string {
	if t.Bungee() {
		return "config.yml"
	}
	return "velocity.toml"
}

// MaintenanceFolder returns the folder in the data of a proxy in which the Maintenance
// plugin by kennytv keeps its files.
func (t ServerType) MaintenanceFolder() string {
	if t.Bungee() {
		return "plugins/Maintenance"
	}
	return "plugins/maintenance"
}
