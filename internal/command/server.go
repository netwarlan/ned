package command

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/netwarlan/ned/internal/config"
	"github.com/netwarlan/ned/internal/executor"
	"github.com/netwarlan/ned/internal/query"
)

// ServerHandler handles /ned server commands.
type ServerHandler struct {
	cfg      *config.Config
	executor executor.Executor
	querier  query.Querier
	locks    sync.Map // per-server mutexes
}

func NewServerHandler(cfg *config.Config, exec executor.Executor, querier query.Querier) *ServerHandler {
	return &ServerHandler{
		cfg:      cfg,
		executor: exec,
		querier:  querier,
	}
}

func (h *ServerHandler) serverLock(key string) *sync.Mutex {
	val, _ := h.locks.LoadOrStore(key, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// Subcommands returns the start, stop, restart, and status subcommands for /ned.
func (h *ServerHandler) Subcommands() []*discordgo.ApplicationCommandOption {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0, len(h.cfg.Servers))
	for key, srv := range h.cfg.Servers {
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  srv.DisplayName,
			Value: key,
		})
	}
	sort.Slice(choices, func(i, j int) bool { return choices[i].Name < choices[j].Name })

	serviceOption := func() *discordgo.ApplicationCommandOption {
		return &discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionString,
			Name:        "service",
			Description: "The game server to manage",
			Required:    true,
			Choices:     choices,
		}
	}

	return []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "start",
			Description: "Start a game server",
			Options:     []*discordgo.ApplicationCommandOption{serviceOption()},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "stop",
			Description: "Stop a game server",
			Options:     []*discordgo.ApplicationCommandOption{serviceOption()},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "restart",
			Description: "Restart a game server",
			Options:     []*discordgo.ApplicationCommandOption{serviceOption()},
		},
		{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "status",
			Description: "Show status of all game servers",
		},
	}
}

// HandleStart handles /ned start <service>.
func (h *ServerHandler) HandleStart(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	h.handleLifecycle(s, i, sub, "up")
}

// HandleStop handles /ned stop <service>.
func (h *ServerHandler) HandleStop(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	h.handleLifecycle(s, i, sub, "down")
}

// HandleRestart handles /ned restart <service>.
func (h *ServerHandler) HandleRestart(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	h.handleLifecycle(s, i, sub, "restart")
}

// HandleStatus handles /ned status.
func (h *ServerHandler) HandleStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	h.handleStatus(s, i)
}

func (h *ServerHandler) handleLifecycle(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption, action string) {
	serviceKey := sub.Options[0].StringValue()
	srv, ok := h.cfg.Servers[serviceKey]
	if !ok {
		respondNow(s, i, fmt.Sprintf("**Error:** Unknown server: %s", serviceKey), true)
		return
	}

	mu := h.serverLock(serviceKey)
	if !mu.TryLock() {
		respondNow(s, i, fmt.Sprintf("**Error:** %s is already being managed by another command", srv.DisplayName), true)
		return
	}

	// Fire-and-forget: respond immediately and run the script in the background.
	// The game server scripts tail logs forever after starting, so waiting
	// for them to finish would leave Discord stuck on "thinking...".
	actionVerb := map[string]string{"up": "Starting", "down": "Stopping", "restart": "Restarting"}
	verb := actionVerb[action]
	if verb == "" {
		verb = action
	}
	respondNow(s, i, fmt.Sprintf("**%s** %s...", verb, srv.DisplayName), true)

	go func() {
		defer mu.Unlock()
		result, err := h.executor.Run(context.Background(), srv.Script, action, nil)
		if err != nil {
			log.Printf("[%s] %s %s failed: %v", serviceKey, action, srv.DisplayName, err)
			return
		}
		if result.ExitCode != 0 {
			log.Printf("[%s] %s %s exited with code %d", serviceKey, action, srv.DisplayName, result.ExitCode)
		}
	}()
}

func (h *ServerHandler) handleStatus(s *discordgo.Session, i *discordgo.InteractionCreate) {
	respondDeferred(s, i, false)

	targets := h.cfg.AllQueryTargets()

	type statusEntry struct {
		key    string
		status *query.ServerStatus
	}

	var (
		mu      sync.Mutex
		results []statusEntry
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
			results = append(results, statusEntry{key: key, status: status})
			mu.Unlock()
		}(key, addr)
	}
	wg.Wait()

	// Add non-queryable servers as "N/A"
	for key, srv := range h.cfg.Servers {
		if srv.Protocol != "source" {
			results = append(results, statusEntry{
				key:    key,
				status: nil,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool { return results[i].key < results[j].key })

	// Group by category
	categories := map[string][]statusEntry{
		"game":  {},
		"cs2":   {},
		"match": {},
		"infra": {},
	}

	for _, entry := range results {
		if strings.HasPrefix(entry.key, "match-") {
			categories["match"] = append(categories["match"], entry)
		} else if srv, ok := h.cfg.Servers[entry.key]; ok {
			categories[srv.Category] = append(categories[srv.Category], entry)
		}
	}

	embed := &discordgo.MessageEmbed{
		Title:     "NETWAR Server Status",
		Color:     0x00bfff,
		Timestamp: time.Now().Format(time.RFC3339),
		Fields:    []*discordgo.MessageEmbedField{},
	}

	categoryNames := []struct {
		key   string
		label string
	}{
		{"game", "Game Servers"},
		{"cs2", "CS2"},
		{"match", "CS2 Matches"},
		{"infra", "Infrastructure"},
	}

	for _, cat := range categoryNames {
		entries := categories[cat.key]
		if len(entries) == 0 {
			continue
		}

		var lines []string
		for _, e := range entries {
			name := h.cfg.DisplayName(e.key)
			if e.status == nil {
				lines = append(lines, fmt.Sprintf("`%-20s` | N/A", name))
			} else if !e.status.Online {
				lines = append(lines, fmt.Sprintf("`%-20s` | Offline", name))
			} else {
				lines = append(lines, fmt.Sprintf("`%-20s` | `%-16s` | %d/%d",
					name, e.status.Map, e.status.Players, e.status.MaxPlayers))
			}
		}

		embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
			Name:  cat.label,
			Value: strings.Join(lines, "\n"),
		})
	}

	followUpEmbed(s, i, []*discordgo.MessageEmbed{embed})
}
