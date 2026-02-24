package command

import (
	"github.com/bwmarrin/discordgo"
	"github.com/netwarlan/ned/internal/config"
)

// WelcomeHandler handles /ned welcome and /ned tournament commands.
type WelcomeHandler struct {
	cfg *config.Config
}

func NewWelcomeHandler(cfg *config.Config) *WelcomeHandler {
	return &WelcomeHandler{cfg: cfg}
}

// WelcomeSubcommand returns the "welcome" subcommand option.
func (h *WelcomeHandler) WelcomeSubcommand() *discordgo.ApplicationCommandOption {
	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "welcome",
		Description: "Post the event welcome message with all server connection info",
	}
}

// TournamentSubcommand returns the "tournament" subcommand option.
func (h *WelcomeHandler) TournamentSubcommand() *discordgo.ApplicationCommandOption {
	minVal := float64(1)
	maxVal := float64(h.cfg.CS2Matches.Pro.MaxInstances)

	return &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionSubCommand,
		Name:        "tournament",
		Description: "Post CS2 tournament match server connection info",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionInteger,
				Name:        "matches",
				Description: "Number of match servers to list",
				MinValue:    &minVal,
				MaxValue:    maxVal,
			},
		},
	}
}

// HandleWelcome posts the welcome message.
func (h *WelcomeHandler) HandleWelcome(s *discordgo.Session, i *discordgo.InteractionCreate) {
	msg := h.cfg.BuildWelcomeMessage()
	if msg == "" {
		respondDeferred(s, i, true)
		followUpError(s, i, "No welcome message configured. Add a `welcome` section to config.yaml.", nil)
		return
	}

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: msg},
	}); err != nil {
		respondDeferred(s, i, true)
		followUpError(s, i, "Failed to send welcome message", err)
	}
}

// HandleTournament posts the CS2 tournament connection info.
func (h *WelcomeHandler) HandleTournament(s *discordgo.Session, i *discordgo.InteractionCreate, sub *discordgo.ApplicationCommandInteractionDataOption) {
	count := h.cfg.CS2Matches.Pro.MaxInstances
	for _, opt := range sub.Options {
		if opt.Name == "matches" {
			count = int(opt.IntValue())
		}
	}

	msg := h.cfg.BuildTournamentMessage(count)

	if err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{Content: msg},
	}); err != nil {
		respondDeferred(s, i, true)
		followUpError(s, i, "Failed to send tournament message", err)
	}
}
