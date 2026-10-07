package noryxv1

const rconProperty = "The console sends its commands through RCON."

// ManagedProperties are the properties of server.properties that Noryx sets on every game
// server itself, with the reason: changing them would break the server or its console. Neither
// the editor of server.properties nor templates change them.
var ManagedProperties = map[string]string{
	"server-port":   "Inside its container, the server always uses this port. Change the port in the server's settings.",
	"server-ip":     "The server listens on all addresses of its container.",
	"enable-rcon":   rconProperty,
	"rcon.port":     rconProperty,
	"rcon.password": rconProperty,
}

// SecretProperties are never sent to the panel, nor changed through its editor or templates.
var SecretProperties = map[string]bool{"rcon.password": true, "management-server-secret": true, "management-server-tls-keystore-password": true}
