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
	"github.com/netwarlan/ned/internal/query"
)

// PlayersHandler handles /ned players commands.
type PlayersHandler struct {
	cfg     *config.Config
	querier query.Querier
}

func NewPlayersHandler(cfg *config.Config, querier query.Querier) *PlayersHandler {
	return &PlayersHandler{cfg: cfg, querier: querier}
}

// Subcommand returns the "players" subcommand option for the /ned command.
func (h *PlayersHandler) Subcommand() *discordgo.ApplicationCommandOption {
	targets := h.cfg.AllQueryTargets()
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(targets))
	for key := range targets {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  h.cfg.DisplayName(key),
			Value: key,
		})
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].Name < choices[j].Name })

	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "players",
		Description: "Show player counts and connected players",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "server",
				Description: "Specific server to query (default: all)",
				Choices:     choices,
			},
		},
	}
}

// Handle executes /ned players.
// sub is the "players" subcommand option.
func (h *PlayersHandler) Handle(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	respondDeferred(s, i, false)

	if len(sub.Options) > 0 && sub.Options[0].StringValue() != "" {
		h.handleSingleServer(s, i, sub.Options[0].StringValue())
		return
	}
	h.handleAllServers(s, i)
}

func (h *PlayersHandler) handleSingleServer(s *discordgo.Session, i *discordgo.InteractionCreate, serverKey string) {
	targets := h.cfg.AllQueryTargets()
	addr, ok := targets[serverKey]
	if !ok {
		followUpError(s, i, fmt.Sprintf("Server %s is not queryable", serverKey), nil)
		return
	}

	name := h.cfg.DisplayName(serverKey)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	status, err := h.querier.QueryStatus(ctx, addr)
	if err != nil || !status.Online {
		followUpError(s, i, fmt.Sprintf("%s is offline or unreachable", name), err)
		return
	}

	players, _ := h.querier.QueryPlayers(ctx, addr)

	embed := &discordgo.MessageEmbed{
		Title: name,
		Color: 0x00ff00,
		Fields: []*discordgo.MessageEmbedField{
			{Name: "Map", Value: status.Map, Inline: true},
			{Name: "Players", Value: fmt.Sprintf("%d/%d", status.Players, status.MaxPlayers), Inline: true},
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if status.Bots > 0 {
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name: "Bots", Value: fmt.Sprintf("%d", status.Bots), Inline: true,
		})
	}

	if len(players) > 0 {
		var lines []string
		for _, p := range players {
			dur := p.Duration.Truncate(time.Second)
			lines = append(lines, fmt.Sprintf("`%-20s` | Score: %d | %s", p.Name, p.Score, dur))
		}
		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  "Connected Players",
			Value: truncate(strings.Join(lines, "\n"), 1024),
		})
	}

	followUpEmbed(s, i, []*discordgo.MessageEmbed{embed})
}

func (h *PlayersHandler) handleAllServers(s *discordgo.Session, i *discordgo.InteractionCreate) {
	targets := h.cfg.AllQueryTargets()

	type entry struct {
		key    string
		status *query.ServerStatus
	}

	var (
		mu      sync.Mutex
		results []entry
		wg      sync.WaitGroup
	)

	for key, addr := range targets {
		wg.Add(1)
		go func(key, addr string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			status, _ := h.querier.QueryStatus(ctx, addr)
			if status == nil {
				status = &query.ServerStatus{Online: false}
			}
			mu.Lock()
			results = append(results, entry{key: key, status: status})
			mu.Unlock()
		}(key, addr)
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool { return results[i].key < results[j].key })

	totalPlayers := 0
	var lines []string
	for _, e := range results {
		name := h.cfg.DisplayName(e.key)
		if !e.status.Online {
			continue
		}
		totalPlayers += e.status.Players
		lines = append(lines, fmt.Sprintf("`%-20s` | `%-16s` | **%d**/%d",
			name, e.status.Map, e.status.Players, e.status.MaxPlayers))
	}

	description := "No servers are currently online."
	if len(lines) > 0 {
		description = strings.Join(lines, "\n")
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("NETWAR Players (%d total)", totalPlayers),
		Description: truncate(description, 4000),
		Color:       0x00bfff,
		Timestamp:   time.Now().Format(time.RFC3339),
	}

	followUpEmbed(s, i, []*discordgo.MessageEmbed{embed})
}
