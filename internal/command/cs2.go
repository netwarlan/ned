package command

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/netwarlan/ned/internal/config"
	"github.com/netwarlan/ned/internal/executor"
	"github.com/netwarlan/ned/internal/rcon"
)

// CS2Handler handles /ned match and /ned map commands.
type CS2Handler struct {
	cfg     *config.Config
	match   *executor.MatchExecutor
	rcon    rcon.Client
	matchMu sync.Mutex // serializes match start/stop operations
}

func NewCS2Handler(cfg *config.Config, match *executor.MatchExecutor, rcon rcon.Client) *CS2Handler {
	return &CS2Handler{
		cfg:   cfg,
		match: match,
		rcon:  rcon,
	}
}

// MatchSubcommandGroup returns the "match" subcommand group for the /ned command.
func (h *CS2Handler) MatchSubcommandGroup() *discordgo.ApplicationCommandOption {
	minCount := float64(1)
	maxCount := float64(h.cfg.CS2Matches.Pro.MaxInstances)

	targets := h.cfg.AllCS2RCONTargets()
	serverChoices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(targets))
	for key := range targets {
		serverChoices = append(serverChoices, &discordgo.ApplicationCommandOptionChoice{
			Name:  h.cfg.DisplayName(key),
			Value: key,
		})
	}
	sort.Slice(serverChoices, func(i, j int) bool { return serverChoices[i].Name < serverChoices[j].Name })

	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommandGroup,
		Name:        "match",
		Description: "Manage CS2 match servers",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "start",
				Description: "Start CS2 match servers",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionInteger,
						Name:        "count",
						Description: fmt.Sprintf("Number of match servers (1-%d)", h.cfg.CS2Matches.Pro.MaxInstances),
						Required:    true,
						MinValue:    &minCount,
						MaxValue:    maxCount,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "stop",
				Description: "Stop all CS2 match servers",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "map",
				Description: "Change map on CS2 servers via RCON",
				Options: []*discordgo.ApplicationCommandOption{
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "map_name",
						Description: "Map to change to (e.g., de_dust2, de_mirage)",
						Required:    true,
					},
					{
						Type:        discordgo.ApplicationCommandOptionString,
						Name:        "server",
						Description: "Target a specific server (default: all CS2 servers)",
						Choices:     serverChoices,
					},
				},
			},
		},
	}
}

// HandleMatch dispatches /ned match subcommands.
// sub is the "match" subcommand group option.
func (h *CS2Handler) HandleMatch(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	action := sub.Options[0]
	switch action.Name {
	case "start":
		h.handleMatchStart(s, i, action)
	case "stop":
		h.handleMatchStop(s, i)
	case "map":
		h.handleMap(s, i, action)
	}
}

func (h *CS2Handler) handleMap(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	respondDeferred(s, i, true)

	var mapName, serverKey string
	for _, opt := range sub.Options {
		switch opt.Name {
		case "map_name":
			mapName = opt.StringValue()
		case "server":
			serverKey = opt.StringValue()
		}
	}

	targets := h.cfg.AllCS2RCONTargets()

	if serverKey != "" {
		target, ok := targets[serverKey]
		if !ok {
			followUpError(s, i, fmt.Sprintf("Unknown CS2 server: %s", serverKey), nil)
			return
		}
		targets = map[string]config.RCONTarget{serverKey: target}
	}

	command := "changelevel " + mapName

	type rconResult struct {
		server   string
		response string
		err      error
	}

	var (
		mu      sync.Mutex
		results []rconResult
		wg      sync.WaitGroup
	)

	for key, target := range targets {
		wg.Add(1)
		go func(key string, target config.RCONTarget) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			resp, err := h.rcon.Execute(ctx, target.Address, target.Password, command)
			mu.Lock()
			results = append(results, rconResult{server: key, response: resp, err: err})
			mu.Unlock()
		}(key, target)
	}
	wg.Wait()

	var lines []string
	for _, r := range results {
		name := h.cfg.DisplayName(r.server)
		if r.err != nil {
			lines = append(lines, fmt.Sprintf("%s: **failed** - %s", name, r.err.Error()))
		} else {
			lines = append(lines, fmt.Sprintf("%s: changed to `%s`", name, mapName))
		}
	}

	msg := fmt.Sprintf("**Map change: %s**\n%s", mapName, strings.Join(lines, "\n"))
	followUp(s, i, msg)
}

func (h *CS2Handler) handleMatchStart(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	respondDeferred(s, i, true)

	count := int(sub.Options[0].IntValue())

	if !h.matchMu.TryLock() {
		followUpError(s, i, "A match operation is already in progress", nil)
		return
	}
	defer h.matchMu.Unlock()

	result, err := h.match.Start(context.Background(), count)
	if err != nil {
		followUpError(s, i, "Failed to start match servers", err)
		return
	}

	msg := fmt.Sprintf("**Started %d CS2 match server(s)**", count)
	if result.Stdout != "" {
		msg += fmt.Sprintf("\n```\n%s\n```", truncate(result.Stdout, maxMessageLen))
	}
	followUp(s, i, msg)
}

func (h *CS2Handler) handleMatchStop(s *discordgo.Session, i *discordgo.InteractionCreate) {
	respondDeferred(s, i, true)

	if !h.matchMu.TryLock() {
		followUpError(s, i, "A match operation is already in progress", nil)
		return
	}
	defer h.matchMu.Unlock()

	result, err := h.match.Stop(context.Background())
	if err != nil {
		followUpError(s, i, "Failed to stop match servers", err)
		return
	}

	msg := "**Stopped all CS2 match servers**"
	if result.Stdout != "" {
		msg += fmt.Sprintf("\n```\n%s\n```", truncate(result.Stdout, maxMessageLen))
	}
	followUp(s, i, msg)
}
