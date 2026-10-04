package network

import (
	"reflect"
	"strings"
	"testing"
)

func TestSettingsVelocity(t *testing.T) {
	settings, locked, err := velocity.Settings([]byte(defaultVelocity))
	if err != nil {
		t.Fatal(err)
	}
	wantSettings := map[string]string{"motd": `"<#09add3>My Network"`, "advanced.compression-threshold": "256"}
	if !reflect.DeepEqual(settings, wantSettings) {
		t.Errorf("settings = %v, want %v", settings, wantSettings)
	}
	for _, key := range []string{"config-version", "bind", "servers", "forced-hosts", "player-info-forwarding-mode", "forwarding-secret-file"} {
		if locked[key] == "" {
			t.Errorf("%s isn't locked", key)
		}
	}
}

func TestSettingsBungee(t *testing.T) {
	settings, locked, err := bungee.Settings([]byte(defaultBungee))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"online_mode": "true", "listeners.0.motd": `"&1Another Bungee server"`, "listeners.0.max_players": "1",
		"listeners.0.query_port": "25577", "permissions.default": `["bungeecord.command.server"]`,
	} {
		if settings[key] != value {
			t.Errorf("%s = %s, want %s", key, settings[key], value)
		}
	}
	for _, key := range []string{"servers", "ip_forward", "listeners.0.host", "listeners.0.priorities", "listeners.0.forced_hosts"} {
		if locked[key] == "" || settings[key] != "" {
			t.Errorf("%s isn't locked", key)
		}
	}
}

func TestChange(t *testing.T) {
	config, err := bungee.Change([]byte(defaultBungee), map[string]string{
		"online_mode": "false", "listeners.0.max_players": "500", "listeners.0.motd": `"&aHello"`,
		"permissions.default": `["bungeecord.command.server", "bungeecord.command.list"]`,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := parse("config.yml", config)
	want(t, got, map[string]any{
		"online_mode": false, "permissions/default": []any{"bungeecord.command.server", "bungeecord.command.list"},
		"servers/lobby/address": "localhost:25565",
	})
	want(t, got["listeners"].([]any)[0].(map[string]any), map[string]any{"max_players": 500, "motd": "&aHello", "host": "0.0.0.0:25577"})

	for _, tt := range []struct{ key, value, err string }{
		{"ip_forward", "true", "can't be changed"},
		{"listeners.0.host", `"0.0.0.0:1"`, "can't be changed"},
		{"servers.lobby.address", `"evil:1"`, "can't be changed"},
		{"unknown", "1", "has no setting"},
		{"online_mode", `"yes"`, "on or off"},
		{"listeners.0.max_players", "1.5", "whole number"},
		{"listeners.0.motd", `"\u0007"`, "control characters"},
		{"permissions.default", `[1]`, "entries"},
	} {
		if _, err := bungee.Change([]byte(defaultBungee), map[string]string{tt.key: tt.value}); err == nil || !strings.Contains(err.Error(), tt.err) {
			t.Errorf("%s = %s: err = %v, want %q", tt.key, tt.value, err, tt.err)
		}
	}

	config, err = velocity.Change([]byte(defaultVelocity), map[string]string{"advanced.compression-threshold": "-1"})
	if err != nil {
		t.Fatal(err)
	}
	got, _ = parse("velocity.toml", config)
	want(t, got, map[string]any{"advanced/compression-threshold": int64(-1), "bind": "0.0.0.0:25565"})
}
