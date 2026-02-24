package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	content := `
discord:
  token: "test-token"
  guild_id: "123456"
scripts_dir: "/scripts"
environment: "event"
servers:
  tf2:
    display_name: "TF2"
    script: "tf2/tf2.sh"
    protocol: "source"
    rcon_password: "secret"
    category: "game"
    event:
      ip: "10.10.10.122"
      port: 27015
      query_port: 27015
      rcon_port: 27015
  satisfactory:
    display_name: "Satisfactory"
    script: "satisfactory/satisfactory.sh"
    protocol: "none"
    category: "game"
    event:
      ip: "10.10.10.124"
      port: 7777
cs2_matches:
  script: "cs2/match/match.sh"
  rcon_password: "headshot"
  rcon_port: 27015
  query_port: 27015
  protocol: "source"
  pro:
    max_instances: 10
    ip_base: "10.10.10.140"
    cpu_base: 17
    display_prefix: "CS2 Match"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Discord.Token != "test-token" {
		t.Errorf("token = %q, want %q", cfg.Discord.Token, "test-token")
	}
	if cfg.Environment != "event" {
		t.Errorf("environment = %q, want %q", cfg.Environment, "event")
	}
	if cfg.ResolvedScriptsDir != "/scripts" {
		t.Errorf("scripts_dir = %q, want %q", cfg.ResolvedScriptsDir, "/scripts")
	}
	if len(cfg.Servers) != 2 {
		t.Errorf("len(servers) = %d, want 2", len(cfg.Servers))
	}
	if cfg.Servers["tf2"].DisplayName != "TF2" {
		t.Errorf("tf2 display_name = %q, want %q", cfg.Servers["tf2"].DisplayName, "TF2")
	}
	if cfg.Servers["tf2"].IP != "10.10.10.122" {
		t.Errorf("tf2 ip = %q, want %q", cfg.Servers["tf2"].IP, "10.10.10.122")
	}
	if cfg.Servers["tf2"].Port != 27015 {
		t.Errorf("tf2 port = %d, want 27015", cfg.Servers["tf2"].Port)
	}
}

func TestLoad_EnvExpansion(t *testing.T) {
	t.Setenv("TEST_BOT_TOKEN", "expanded-token")

	content := `
discord:
  token: "${TEST_BOT_TOKEN}"
  guild_id: "123456"
scripts_dir: "/scripts"
environment: "event"
servers: {}
cs2_matches:
  script: "cs2/match/match.sh"
  rcon_password: "pw"
  rcon_port: 27015
  protocol: "source"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Discord.Token != "expanded-token" {
		t.Errorf("token = %q, want %q", cfg.Discord.Token, "expanded-token")
	}
}

func TestLoad_PerEnvironmentScriptsDir(t *testing.T) {
	content := `
discord:
  token: "tok"
  guild_id: "123456"
scripts_dir:
  event: "/home/docker/netwar46.0"
  local: "/docker/game-deployment-scripts"
environment: "local"
servers: {}
cs2_matches:
  script: "cs2/cs2.sh"
  rcon_password: "pw"
  rcon_port: 27015
  protocol: "source"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ResolvedScriptsDir != "/docker/game-deployment-scripts" {
		t.Errorf("scripts_dir = %q, want %q", cfg.ResolvedScriptsDir, "/docker/game-deployment-scripts")
	}
}

func TestValidate_MissingToken(t *testing.T) {
	cfg := &Config{
		Discord:            DiscordConfig{GuildID: "123"},
		ResolvedScriptsDir: "/scripts",
		Environment:        "event",
		CS2Matches:         CS2MatchConfig{Script: "match.sh"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing token")
	}
}

func TestValidate_InvalidEnvironment(t *testing.T) {
	cfg := &Config{
		Discord:            DiscordConfig{Token: "tok", GuildID: "123"},
		ResolvedScriptsDir: "/scripts",
		Environment:        "production",
		CS2Matches:         CS2MatchConfig{Script: "match.sh"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for invalid environment")
	}
}

func TestMatchTierConfig_InstanceIP(t *testing.T) {
	tier := MatchTierConfig{IPBase: "10.10.10.140"}

	tests := []struct {
		n    int
		want string
	}{
		{1, "10.10.10.141"},
		{5, "10.10.10.145"},
		{10, "10.10.10.150"},
	}
	for _, tt := range tests {
		got := tier.InstanceIP(tt.n)
		if got != tt.want {
			t.Errorf("InstanceIP(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestQueryableServers(t *testing.T) {
	cfg := &Config{
		Servers: map[string]Server{
			"tf2":          {Protocol: "source"},
			"satisfactory": {Protocol: "none"},
			"rust":         {Protocol: "source"},
		},
	}
	qs := cfg.QueryableServers()
	if len(qs) != 2 {
		t.Errorf("len(queryable) = %d, want 2", len(qs))
	}
	if _, ok := qs["satisfactory"]; ok {
		t.Error("satisfactory should not be queryable")
	}
}

func TestRCONCapableServers(t *testing.T) {
	cfg := &Config{
		Servers: map[string]Server{
			"tf2":          {RCONPort: 27015, RCONPassword: "secret"},
			"satisfactory": {RCONPort: 0, RCONPassword: ""},
		},
	}
	rs := cfg.RCONCapableServers()
	if len(rs) != 1 {
		t.Errorf("len(rcon_capable) = %d, want 1", len(rs))
	}
}

func TestAllCS2RCONTargets(t *testing.T) {
	cfg := &Config{
		Servers: map[string]Server{
			"cs2-casual": {Category: "cs2", RCONPort: 27015, RCONPassword: "headshot", IP: "10.10.10.131"},
		},
		CS2Matches: CS2MatchConfig{
			RCONPassword: "headshot",
			RCONPort:     27015,
			Pro:          MatchTierConfig{MaxInstances: 2, IPBase: "10.10.10.140"},
		},
	}

	targets := cfg.AllCS2RCONTargets()
	// 1 static + 2 pro = 3
	if len(targets) != 3 {
		t.Errorf("len(targets) = %d, want 3", len(targets))
	}
	if targets["match-pro-1"].Address != "10.10.10.141:27015" {
		t.Errorf("match-pro-1 address = %q, want %q", targets["match-pro-1"].Address, "10.10.10.141:27015")
	}
}

func TestEnvironmentResolution_Event(t *testing.T) {
	content := `
discord:
  token: "tok"
  guild_id: "123456"
scripts_dir:
  event: "/home/docker/netwar46.0"
  local: "/docker/game-deployment-scripts"
environment: "event"
servers:
  tf2:
    display_name: "TF2"
    script: "tf2/tf2.sh"
    protocol: "source"
    rcon_password: "secret"
    category: "game"
    event:
      ip: "10.10.10.122"
      port: 27015
      query_port: 27015
      rcon_port: 27015
    local:
      ip: "127.0.0.1"
      port: 27015
      query_port: 27015
      rcon_port: 27015
  garrysmod:
    display_name: "Garry's Mod"
    script: "garrysmod/garrysmod.sh"
    protocol: "source"
    rcon_password: ""
    category: "game"
    event:
      ip: "10.10.10.121"
      port: 27015
      query_port: 27015
      rcon_port: 27015
    local:
      ip: "127.0.0.1"
      port: 27016
      query_port: 27016
      rcon_port: 27016
cs2_matches:
  script: "cs2/cs2.sh"
  rcon_password: "pw"
  rcon_port: 27015
  protocol: "source"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ResolvedScriptsDir != "/home/docker/netwar46.0" {
		t.Errorf("scripts_dir = %q, want %q", cfg.ResolvedScriptsDir, "/home/docker/netwar46.0")
	}
	if cfg.Servers["tf2"].IP != "10.10.10.122" {
		t.Errorf("tf2 ip = %q, want %q", cfg.Servers["tf2"].IP, "10.10.10.122")
	}
	if cfg.Servers["garrysmod"].IP != "10.10.10.121" {
		t.Errorf("garrysmod ip = %q, want %q", cfg.Servers["garrysmod"].IP, "10.10.10.121")
	}
	if cfg.Servers["garrysmod"].Port != 27015 {
		t.Errorf("garrysmod port = %d, want 27015", cfg.Servers["garrysmod"].Port)
	}
}

func TestEnvironmentResolution_Local(t *testing.T) {
	content := `
discord:
  token: "tok"
  guild_id: "123456"
scripts_dir:
  event: "/home/docker/netwar46.0"
  local: "/docker/game-deployment-scripts"
environment: "local"
servers:
  tf2:
    display_name: "TF2"
    script: "tf2/tf2.sh"
    protocol: "source"
    rcon_password: "secret"
    category: "game"
    event:
      ip: "10.10.10.122"
      port: 27015
      query_port: 27015
      rcon_port: 27015
    local:
      ip: "127.0.0.1"
      port: 27015
      query_port: 27015
      rcon_port: 27015
  garrysmod:
    display_name: "Garry's Mod"
    script: "garrysmod/garrysmod.sh"
    protocol: "source"
    rcon_password: ""
    category: "game"
    event:
      ip: "10.10.10.121"
      port: 27015
      query_port: 27015
      rcon_port: 27015
    local:
      ip: "127.0.0.1"
      port: 27016
      query_port: 27016
      rcon_port: 27016
  minecraft:
    display_name: "Minecraft"
    script: "minecraft/minecraft.sh"
    protocol: "none"
    category: "game"
    event:
      ip: "10.10.10.130"
      port: 25565
    local:
      ip: "127.0.0.1"
      port: 25565
cs2_matches:
  script: "cs2/cs2.sh"
  rcon_password: "pw"
  rcon_port: 27015
  protocol: "source"
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ResolvedScriptsDir != "/docker/game-deployment-scripts" {
		t.Errorf("scripts_dir = %q, want %q", cfg.ResolvedScriptsDir, "/docker/game-deployment-scripts")
	}

	// All servers should have local IP
	if cfg.Servers["tf2"].IP != "127.0.0.1" {
		t.Errorf("tf2 ip = %q, want %q", cfg.Servers["tf2"].IP, "127.0.0.1")
	}
	if cfg.Servers["garrysmod"].IP != "127.0.0.1" {
		t.Errorf("garrysmod ip = %q, want %q", cfg.Servers["garrysmod"].IP, "127.0.0.1")
	}

	// Garry's Mod remapped to 27016
	if cfg.Servers["garrysmod"].Port != 27016 {
		t.Errorf("garrysmod port = %d, want 27016", cfg.Servers["garrysmod"].Port)
	}
	if cfg.Servers["garrysmod"].QueryPort != 27016 {
		t.Errorf("garrysmod query_port = %d, want 27016", cfg.Servers["garrysmod"].QueryPort)
	}

	// Minecraft keeps its port
	if cfg.Servers["minecraft"].Port != 25565 {
		t.Errorf("minecraft port = %d, want 25565", cfg.Servers["minecraft"].Port)
	}
}

func TestDisplayName(t *testing.T) {
	cfg := &Config{
		Servers: map[string]Server{
			"tf2": {DisplayName: "TF2 Casual"},
		},
		CS2Matches: CS2MatchConfig{
			Pro: MatchTierConfig{DisplayPrefix: "CS2 Match"},
		},
	}

	tests := []struct {
		key  string
		want string
	}{
		{"tf2", "TF2 Casual"},
		{"match-pro-3", "CS2 Match 3"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		got := cfg.DisplayName(tt.key)
		if got != tt.want {
			t.Errorf("DisplayName(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}
