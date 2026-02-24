package bot

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/netwarlan/ned/internal/command"
	"github.com/netwarlan/ned/internal/config"
	"github.com/netwarlan/ned/internal/executor"
	"github.com/netwarlan/ned/internal/query"
	"github.com/netwarlan/ned/internal/rcon"
)

// Bot is the top-level Discord bot that owns the session and command handlers.
type Bot struct {
	cfg     *config.Config
	version string
	session *discordgo.Session

	serverHandler  *command.ServerHandler
	cs2Handler     *command.CS2Handler
	rconHandler    *command.RCONHandler
	playersHandler *command.PlayersHandler
	welcomeHandler *command.WelcomeHandler

	registeredCommand *discordgo.ApplicationCommand
}

// New creates a new Bot instance with all dependencies wired up.
func New(cfg *config.Config, version string) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.Discord.Token)
	if err != nil {
		return nil, err
	}

	exec := executor.NewShellExecutor(cfg.ScriptsDir, cfg.Environment)
	matchExec := executor.NewMatchExecutor(
		exec,
		cfg.CS2Matches.Script,
		cfg.CS2Matches.Pro.MaxInstances,
		cfg.CS2Matches.Casual.MaxInstances,
	)
	querier := query.NewA2SQuerier(5 * time.Second)
	rconClient := rcon.NewGorconClient(10 * time.Second)

	return &Bot{
		cfg:            cfg,
		version:        version,
		session:        session,
		serverHandler:  command.NewServerHandler(cfg, exec, querier),
		cs2Handler:     command.NewCS2Handler(cfg, matchExec, rconClient),
		rconHandler:    command.NewRCONHandler(cfg, rconClient),
		playersHandler: command.NewPlayersHandler(cfg, querier),
		welcomeHandler: command.NewWelcomeHandler(cfg),
	}, nil
}

// Start opens the Discord websocket connection and registers the /ned command.
func (b *Bot) Start() error {
	b.session.AddHandler(b.handleInteraction)

	if err := b.session.Open(); err != nil {
		return err
	}

	cmd := b.buildCommand()
	registered, err := b.session.ApplicationCommandCreate(
		b.session.State.User.ID,
		b.cfg.Discord.GuildID,
		cmd,
	)
	if err != nil {
		return err
	}
	b.registeredCommand = registered
	log.Printf("Registered command: /%s", cmd.Name)

	return nil
}

// Stop deregisters the slash command and closes the Discord session.
func (b *Bot) Stop() error {
	if b.registeredCommand != nil {
		if err := b.session.ApplicationCommandDelete(
			b.session.State.User.ID,
			b.cfg.Discord.GuildID,
			b.registeredCommand.ID,
		); err != nil {
			log.Printf("Failed to deregister command: %v", err)
		}
	}
	return b.session.Close()
}

// buildCommand constructs the single /ned command with all subcommands.
func (b *Bot) buildCommand() *discordgo.ApplicationCommand {
	return &discordgo.ApplicationCommand{
		Name:        "ned",
		Description: "NETWAR Event Discord bot — manage game servers",
		Options: []*discordgo.ApplicationCommandOption{
			b.serverHandler.SubcommandGroup(),
			b.cs2Handler.MatchSubcommandGroup(),
			b.cs2Handler.MapSubcommand(),
			b.rconHandler.Subcommand(),
			b.playersHandler.Subcommand(),
			b.welcomeHandler.WelcomeSubcommand(),
			b.welcomeHandler.TournamentSubcommand(),
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "ping",
				Description: "Check if the bot is alive",
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "version",
				Description: "Show the running bot version",
			},
		},
	}
}

// formatOptions builds a human-readable string from a command option tree.
// e.g. "server start service=tf2" or "rcon server=cs2-casual command=status"
func formatOptions(opt *discordgo.ApplicationCommandInteractionDataOption) string {
	var parts []string
	parts = append(parts, opt.Name)
	for _, child := range opt.Options {
		if child.Type == discordgo.ApplicationCommandOptionSubCommand ||
			child.Type == discordgo.ApplicationCommandOptionSubCommandGroup {
			parts = append(parts, formatOptions(child))
		} else {
			parts = append(parts, fmt.Sprintf("%s=%v", child.Name, child.Value))
		}
	}
	return strings.Join(parts, " ")
}


func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	if i.ApplicationCommandData().Name != "ned" {
		return
	}

	sub := i.ApplicationCommandData().Options[0]

	user := "unknown"
	if i.Member != nil && i.Member.User != nil {
		user = i.Member.User.Username
	}
	log.Printf("[command] user=%s cmd=/ned %s", user, formatOptions(sub))

	switch sub.Name {
	case "server":
		b.serverHandler.Handle(s, i, sub)
	case "match":
		b.cs2Handler.HandleMatch(s, i, sub)
	case "map":
		b.cs2Handler.HandleMap(s, i, sub)
	case "rcon":
		b.rconHandler.Handle(s, i, sub)
	case "players":
		b.playersHandler.Handle(s, i, sub)
	case "welcome":
		b.welcomeHandler.HandleWelcome(s, i)
	case "tournament":
		b.welcomeHandler.HandleTournament(s, i, sub)
	case "ping":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: "Pong!"},
		})
	case "version":
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: fmt.Sprintf("Ned %s", b.version)},
		})
	}
}
