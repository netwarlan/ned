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
	opts := []*discordgo.ApplicationCommandOption{}
	opts = append(opts, b.serverHandler.Subcommands()...)
	opts = append(opts,
		b.cs2Handler.MatchSubcommandGroup(),
		b.rconHandler.Subcommand(),
		b.playersHandler.Subcommand(),
		b.welcomeHandler.WelcomeSubcommand(),
		b.welcomeHandler.TournamentSubcommand(),
		&discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "help",
			Description: "Show available commands",
		},
		&discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "ping",
			Description: "Check if the bot is alive",
		},
		&discordgo.ApplicationCommandOption{
			Type:        discordgo.ApplicationCommandOptionSubCommand,
			Name:        "version",
			Description: "Show the running bot version",
		},
	)

	return &discordgo.ApplicationCommand{
		Name:        "ned",
		Description: "NETWAR Event Discord bot — manage game servers",
		Options:     opts,
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
	case "start":
		b.serverHandler.HandleStart(s, i, sub)
	case "stop":
		b.serverHandler.HandleStop(s, i, sub)
	case "restart":
		b.serverHandler.HandleRestart(s, i, sub)
	case "status":
		b.serverHandler.HandleStatus(s, i)
	case "match":
		b.cs2Handler.HandleMatch(s, i, sub)
	case "rcon":
		b.rconHandler.Handle(s, i, sub)
	case "players":
		b.playersHandler.Handle(s, i, sub)
	case "welcome":
		b.welcomeHandler.HandleWelcome(s, i)
	case "tournament":
		b.welcomeHandler.HandleTournament(s, i, sub)
	case "help":
		help := "**Ned — NETWAR Event Discord Bot**\n" +
			"```\n" +
			"/ned start <service>            Start a game server\n" +
			"/ned stop <service>             Stop a game server\n" +
			"/ned restart <service>          Restart a game server\n" +
			"/ned status                     Show all server statuses\n" +
			"/ned match start <count>        Spin up CS2 match instances\n" +
			"/ned match stop                 Tear down all match instances\n" +
			"/ned match map <map> [server]   Change CS2 map via RCON\n" +
			"/ned rcon <server> <command>    Send RCON command\n" +
			"/ned players [server]           Show player counts\n" +
			"/ned welcome                    Post event welcome message\n" +
			"/ned tournament [matches]       Post CS2 tournament info\n" +
			"/ned help                       Show this message\n" +
			"/ned ping                       Pong\n" +
			"/ned version                    Show bot version\n" +
			"```"
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{Content: help, Flags: discordgo.MessageFlagsEphemeral},
		})
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
