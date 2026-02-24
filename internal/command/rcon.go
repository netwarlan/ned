package command

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/netwarlan/ned/internal/config"
	"github.com/netwarlan/ned/internal/rcon"
)

// RCONHandler handles /ned rcon commands.
type RCONHandler struct {
	cfg  *config.Config
	rcon rcon.Client
}

func NewRCONHandler(cfg *config.Config, rcon rcon.Client) *RCONHandler {
	return &RCONHandler{cfg: cfg, rcon: rcon}
}

// Subcommand returns the "rcon" subcommand option for the /ned command.
func (h *RCONHandler) Subcommand() *discordgo.ApplicationCommandOption {
	choices := make([]*discordgo.ApplicationCommandOptionChoice, 0)

	for key, srv := range h.cfg.Servers {
		if srv.RCONPort > 0 && srv.RCONPassword != "" {
			choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
				Name:  srv.DisplayName,
				Value: key,
			})
		}
	}

	for i := 1; i <= h.cfg.CS2Matches.Pro.MaxInstances; i++ {
		key := fmt.Sprintf("match-pro-%d", i)
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  h.cfg.DisplayName(key),
			Value: key,
		})
	}
	for i := 1; i <= h.cfg.CS2Matches.Casual.MaxInstances; i++ {
		key := fmt.Sprintf("match-casual-%d", i)
		choices = append(choices, &discordgo.ApplicationCommandOptionChoice{
			Name:  h.cfg.DisplayName(key),
			Value: key,
		})
	}

	sort.Slice(choices, func(i, j int) bool { return choices[i].Name < choices[j].Name })

	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "rcon",
		Description: "Send an RCON command to a game server",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "server",
				Description: "Target server",
				Required:    true,
				Choices:     choices,
			},
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "command",
				Description: "RCON command to execute",
				Required:    true,
			},
		},
	}
}

// Handle executes /ned rcon.
// sub is the "rcon" subcommand option.
func (h *RCONHandler) Handle(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	respondDeferred(s, i, true) // ephemeral — RCON output may be sensitive

	serverKey := sub.Options[0].StringValue()
	command := sub.Options[1].StringValue()

	address, password, err := h.resolveServer(serverKey)
	if err != nil {
		followUpError(s, i, err.Error(), nil)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	response, err := h.rcon.Execute(ctx, address, password, command)
	if err != nil {
		followUpError(s, i, fmt.Sprintf("RCON failed on %s", h.cfg.DisplayName(serverKey)), err)
		return
	}

	name := h.cfg.DisplayName(serverKey)
	msg := fmt.Sprintf("**RCON** `%s` → %s", command, name)
	if response != "" {
		msg += fmt.Sprintf("\n```\n%s\n```", truncate(response, maxMessageLen))
	} else {
		msg += "\n*No response*"
	}
	followUp(s, i, msg)
}

func (h *RCONHandler) resolveServer(key string) (address, password string, err error) {
	if srv, ok := h.cfg.Servers[key]; ok {
		if srv.RCONPort == 0 || srv.RCONPassword == "" {
			return "", "", fmt.Errorf("server %s does not have RCON configured", key)
		}
		return net.JoinHostPort(srv.IP, strconv.Itoa(srv.RCONPort)), srv.RCONPassword, nil
	}

	targets := h.cfg.AllCS2RCONTargets()
	if target, ok := targets[key]; ok {
		return target.Address, target.Password, nil
	}

	return "", "", fmt.Errorf("unknown server: %s", key)
}
