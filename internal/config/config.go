package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Discord     DiscordConfig        `yaml:"discord"`
	ScriptsDir  string               `yaml:"scripts_dir"`
	Environment string               `yaml:"environment"`
	Servers     map[string]Server    `yaml:"servers"`
	CS2Matches  CS2MatchConfig       `yaml:"cs2_matches"`
	Welcome     WelcomeConfig        `yaml:"welcome"`
}

// WelcomeConfig defines the preformatted welcome message structure.
type WelcomeConfig struct {
	ConnectBaseURL string           `yaml:"connect_base_url"`
	Sections       []WelcomeSection `yaml:"sections"`
}

// WelcomeSection is a titled block in the welcome message.
// Use either Servers (auto-formatted) or Text (custom), not both.
type WelcomeSection struct {
	Title   string               `yaml:"title"`
	Servers []WelcomeServerEntry `yaml:"servers"`
	Text    string               `yaml:"text"`
}

// WelcomeServerEntry references a server for auto-formatted connection info.
type WelcomeServerEntry struct {
	Key        string `yaml:"key"`         // references config.servers for IP/port
	Label      string `yaml:"label"`       // display label (omit for single-server sections)
	IP         string `yaml:"ip"`          // explicit IP (when not using a server key)
	Port       int    `yaml:"port"`        // explicit port (when not using a server key)
	ConnectCmd string `yaml:"connect_cmd"` // override "connect" (e.g., "open" for UT2004)
	NoURL      bool   `yaml:"no_url"`      // skip the connect.netwar.org URL line
}

type DiscordConfig struct {
	Token   string `yaml:"token"`
	GuildID string `yaml:"guild_id"`
}

type Server struct {
	DisplayName  string `yaml:"display_name"`
	Script       string `yaml:"script"`
	IP           string `yaml:"ip"`
	Port         int    `yaml:"port"`
	QueryPort    int    `yaml:"query_port"`
	Protocol     string `yaml:"protocol"`
	RCONPort     int    `yaml:"rcon_port"`
	RCONPassword string `yaml:"rcon_password"`
	Category     string `yaml:"category"`
}

type CS2MatchConfig struct {
	Script       string          `yaml:"script"`
	RCONPassword string          `yaml:"rcon_password"`
	RCONPort     int             `yaml:"rcon_port"`
	QueryPort    int             `yaml:"query_port"`
	Protocol     string          `yaml:"protocol"`
	Pro          MatchTierConfig `yaml:"pro"`
}

type MatchTierConfig struct {
	MaxInstances  int    `yaml:"max_instances"`
	IPBase        string `yaml:"ip_base"`
	CPUBase       int    `yaml:"cpu_base"`
	DisplayPrefix string `yaml:"display_prefix"`
}

// Load reads and validates the config file. Environment variables
// referenced as ${VAR_NAME} in string values are expanded.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	expanded := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Discord.Token == "" {
		return fmt.Errorf("discord.token is required")
	}
	if c.Discord.GuildID == "" {
		return fmt.Errorf("discord.guild_id is required")
	}
	if c.ScriptsDir == "" {
		return fmt.Errorf("scripts_dir is required")
	}
	if c.Environment != "event" && c.Environment != "local" {
		return fmt.Errorf("environment must be \"event\" or \"local\", got %q", c.Environment)
	}
	for name, srv := range c.Servers {
		if srv.Script == "" {
			return fmt.Errorf("server %q: script is required", name)
		}
		if srv.Protocol != "source" && srv.Protocol != "none" {
			return fmt.Errorf("server %q: protocol must be \"source\" or \"none\", got %q", name, srv.Protocol)
		}
	}
	if c.CS2Matches.Script == "" {
		return fmt.Errorf("cs2_matches.script is required")
	}
	return nil
}

// QueryableServers returns servers with protocol "source" that support A2S queries.
func (c *Config) QueryableServers() map[string]Server {
	result := make(map[string]Server)
	for name, srv := range c.Servers {
		if srv.Protocol == "source" {
			result[name] = srv
		}
	}
	return result
}

// RCONCapableServers returns servers that have an RCON port and password configured.
func (c *Config) RCONCapableServers() map[string]Server {
	result := make(map[string]Server)
	for name, srv := range c.Servers {
		if srv.RCONPort > 0 && srv.RCONPassword != "" {
			result[name] = srv
		}
	}
	return result
}

// InstanceIP computes the IP address for a CS2 match instance.
// For instance number n (1-based), it adds n to the base IP's last octet.
func (t *MatchTierConfig) InstanceIP(n int) string {
	ip := net.ParseIP(t.IPBase).To4()
	if ip == nil {
		return ""
	}
	ip[3] += byte(n)
	return ip.String()
}

// ServerChoices returns a sorted list of server keys for Discord autocomplete.
func (c *Config) ServerChoices() []string {
	choices := make([]string, 0, len(c.Servers))
	for name := range c.Servers {
		choices = append(choices, name)
	}
	return choices
}

// CS2ServerKeys returns keys of all CS2-category servers (static, not match instances).
func (c *Config) CS2ServerKeys() []string {
	var keys []string
	for name, srv := range c.Servers {
		if srv.Category == "cs2" {
			keys = append(keys, name)
		}
	}
	return keys
}

// AllCS2RCONTargets returns a map of display name → address:port for all CS2 servers
// that support RCON, including dynamically computed match instances.
func (c *Config) AllCS2RCONTargets() map[string]RCONTarget {
	targets := make(map[string]RCONTarget)

	for name, srv := range c.Servers {
		if srv.Category == "cs2" && srv.RCONPort > 0 && srv.RCONPassword != "" {
			targets[name] = RCONTarget{
				Address:  net.JoinHostPort(srv.IP, strconv.Itoa(srv.RCONPort)),
				Password: srv.RCONPassword,
			}
		}
	}

	for i := 1; i <= c.CS2Matches.Pro.MaxInstances; i++ {
		name := fmt.Sprintf("match-pro-%d", i)
		targets[name] = RCONTarget{
			Address:  net.JoinHostPort(c.CS2Matches.Pro.InstanceIP(i), strconv.Itoa(c.CS2Matches.RCONPort)),
			Password: c.CS2Matches.RCONPassword,
		}
	}

	return targets
}

// RCONTarget holds connection details for an RCON-capable server.
type RCONTarget struct {
	Address  string
	Password string
}

// AllQueryTargets returns a map of display name → address for all A2S-queryable servers,
// including dynamically computed match instances.
func (c *Config) AllQueryTargets() map[string]string {
	targets := make(map[string]string)

	for name, srv := range c.Servers {
		if srv.Protocol == "source" && srv.QueryPort > 0 {
			targets[name] = net.JoinHostPort(srv.IP, strconv.Itoa(srv.QueryPort))
		}
	}

	if c.CS2Matches.Protocol == "source" && c.CS2Matches.QueryPort > 0 {
		port := strconv.Itoa(c.CS2Matches.QueryPort)
		for i := 1; i <= c.CS2Matches.Pro.MaxInstances; i++ {
			name := fmt.Sprintf("match-pro-%d", i)
			targets[name] = net.JoinHostPort(c.CS2Matches.Pro.InstanceIP(i), port)
		}
	}

	return targets
}

// BuildWelcomeMessage generates the welcome message from config data.
func (c *Config) BuildWelcomeMessage() string {
	var sb strings.Builder

	for i, section := range c.Welcome.Sections {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("**")
		sb.WriteString(section.Title)
		sb.WriteString("**\n")

		if section.Text != "" {
			sb.WriteString(section.Text)
			if !strings.HasSuffix(section.Text, "\n") {
				sb.WriteString("\n")
			}
			continue
		}

		for j, entry := range section.Servers {
			if j > 0 {
				sb.WriteString("\n")
			}

			ip, port := entry.IP, entry.Port
			if entry.Key != "" {
				if srv, ok := c.Servers[entry.Key]; ok {
					ip = srv.IP
					port = srv.Port
				}
			}

			connectCmd := "connect"
			if entry.ConnectCmd != "" {
				connectCmd = entry.ConnectCmd
			}

			if entry.Label != "" {
				if !entry.NoURL && c.Welcome.ConnectBaseURL != "" {
					sb.WriteString(fmt.Sprintf("%s : %s/?%s:%d\n", entry.Label, c.Welcome.ConnectBaseURL, ip, port))
				} else {
					sb.WriteString(fmt.Sprintf("%s :\n", entry.Label))
				}
			} else if !entry.NoURL && c.Welcome.ConnectBaseURL != "" {
				sb.WriteString(fmt.Sprintf("%s/?%s:%d\n", c.Welcome.ConnectBaseURL, ip, port))
			}

			sb.WriteString(fmt.Sprintf("In game: `%s %s`\n", connectCmd, ip))
		}
	}

	return sb.String()
}

// BuildTournamentMessage generates the CS2 tournament match connection info.
func (c *Config) BuildTournamentMessage(count int) string {
	if count <= 0 {
		count = c.CS2Matches.Pro.MaxInstances
	}
	if count > c.CS2Matches.Pro.MaxInstances {
		count = c.CS2Matches.Pro.MaxInstances
	}

	var sb strings.Builder
	sb.WriteString("**CS2 Tournament**\n")

	for i := 1; i <= count; i++ {
		if i > 1 {
			sb.WriteString("\n")
		}
		ip := c.CS2Matches.Pro.InstanceIP(i)
		port := c.CS2Matches.RCONPort
		if c.Welcome.ConnectBaseURL != "" {
			sb.WriteString(fmt.Sprintf("MATCH %d : %s/?%s:%d\n", i, c.Welcome.ConnectBaseURL, ip, port))
		} else {
			sb.WriteString(fmt.Sprintf("MATCH %d :\n", i))
		}
		sb.WriteString(fmt.Sprintf("In game: `connect %s`\n", ip))
	}

	return sb.String()
}

// DisplayName returns the display name for a server key. For static servers,
// it uses the configured display_name. For match instances, it generates one
// from the key (e.g., "match-pro-1" → "CS2 Match Pro 1").
func (c *Config) DisplayName(key string) string {
	if srv, ok := c.Servers[key]; ok {
		return srv.DisplayName
	}
	if strings.HasPrefix(key, "match-pro-") {
		n := strings.TrimPrefix(key, "match-pro-")
		return c.CS2Matches.Pro.DisplayPrefix + " " + n
	}
	return key
}
