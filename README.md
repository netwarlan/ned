# Ned

NETWAR Event Discord bot — manage game servers from Discord instead of SSH.

Ned wraps the existing [game-deployment-scripts](https://github.com/netwarlan/game-deployment-scripts) shell scripts and adds native RCON and A2S server querying, all behind a single `/ned` slash command.

## Commands

```
/ned start <service>            — start a game server
/ned stop <service>             — stop a game server
/ned restart <service>          — restart a game server
/ned status                     — show all server statuses
/ned match start <count>        — spin up CS2 match instances
/ned match stop                 — tear down all match instances
/ned match map <map> [server]   — change CS2 map via RCON
/ned rcon <server> <command>    — send RCON command
/ned players [server]           — show player counts
/ned welcome                    — post event welcome message
/ned tournament [matches]       — post CS2 tournament info
/ned help                       — show available commands
/ned ping                       — pong
/ned version                    — show bot version
```

## Setup

### Discord Bot

1. Create a bot at [discord.com/developers](https://discord.com/developers/applications)
2. OAuth2 scopes: `bot`, `applications.commands`
3. Bot permissions: `Send Messages`
4. Invite to your server

### Configuration

Copy `config.yaml` and fill in your server definitions. The Discord token is read from the `DISCORD_TOKEN` environment variable:

```yaml
discord:
  token: "${DISCORD_TOKEN}"
  guild_id: "your-guild-id"
```

See [config.yaml](config.yaml) for the full example with all server entries, CS2 match config, and welcome message sections.

### Run Locally

```bash
# Create a .env file (gitignored)
echo 'DISCORD_TOKEN=your-token-here' > .env

# Build and run
make build
export $(grep -v '^#' .env | xargs) && ./ned --config config.yaml
```

### Deploy to Server

Ned runs as a native binary managed by systemd. From `game-deployment-scripts/ned/`:

```bash
# Download the latest release
./ned.sh update

# Install the systemd service (one-time)
./ned.sh install

# Start
./ned.sh up
```

Management commands:

```bash
./ned.sh up        # start the bot
./ned.sh down      # stop the bot
./ned.sh restart   # restart the bot
./ned.sh update    # download latest release and restart
./ned.sh status    # show service status
./ned.sh logs      # view logs
```

## Development

```bash
make build    # build binary (embeds git version/commit/date)
make test     # run tests with race detection
make lint     # run golangci-lint
make clean    # remove binary
```

## CI/CD

Tagging a version (e.g., `git tag v1.0.0 && git push --tags`) triggers a GitHub Actions workflow that builds a Linux binary and publishes it as a GitHub Release.
