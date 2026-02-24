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
    ip: "10.10.10.122"
    port: 27015
    query_port: 27015
    protocol: "source"
    rcon_port: 27015
    rcon_password: "secret"
    category: "game"
  satisfactory:
    display_name: "Satisfactory"
    script: "satisfactory/satisfactory.sh"
    ip: "10.10.10.124"
    port: 7777
    protocol: "none"
    category: "game"
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
	if len(cfg.Servers) != 2 {
		t.Errorf("len(servers) = %d, want 2", len(cfg.Servers))
	}
	if cfg.Servers["tf2"].DisplayName != "TF2" {
		t.Errorf("tf2 display_name = %q, want %q", cfg.Servers["tf2"].DisplayName, "TF2")
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

func TestValidate_MissingToken(t *testing.T) {
	cfg := &Config{
		Discord:     DiscordConfig{GuildID: "123"},
		ScriptsDir:  "/scripts",
		Environment: "event",
		CS2Matches:  CS2MatchConfig{Script: "match.sh"},
	}
	if err := cfg.Validate(); err == nil {
		t.Error("expected error for missing token")
	}
}

func TestValidate_InvalidEnvironment(t *testing.T) {
	cfg := &Config{
		Discord:     DiscordConfig{Token: "tok", GuildID: "123"},
		ScriptsDir:  "/scripts",
		Environment: "production",
		CS2Matches:  CS2MatchConfig{Script: "match.sh"},
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
