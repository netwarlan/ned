# Ned

NETWAR Event Discord bot — manage game servers from Discord instead of SSH.

Ned wraps the existing [game-deployment-scripts](https://github.com/netwarlan/game-deployment-scripts) shell scripts and adds native RCON and A2S server querying, all behind a single `/ned` slash command.

## Commands

```
/ned server start <service>     — start a game server
/ned server stop <service>      — stop a game server
/ned server restart <service>   — restart a game server
/ned server status              — show all server statuses (A2S query)
/ned match start [pro] [casual] — spin up CS2 match instances
/ned match stop                 — tear down all match instances
/ned map <map_name> [server]    — change CS2 map via RCON
/ned rcon <server> <command>    — send RCON command (ephemeral response)
/ned players [server]           — show player counts and connected players
/ned welcome                    — post event welcome message with server connection info
/ned tournament [matches]       — post CS2 tournament match connection info
/ned ping                       — pong
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

### Docker Deployment

```bash
# Event deployment (MAC VLAN)
docker compose -f compose.yaml -f compose.event.yaml up -d

# Local deployment
docker compose -f compose.yaml -f compose.local.yaml up -d
```

The container needs:
- `/var/run/docker.sock` mounted — the bot executes `docker compose` via the host daemon
- `game-deployment-scripts` directory mounted at `/scripts`
- `config.yaml` mounted at `/config/config.yaml`
- `DISCORD_TOKEN` set via environment or `.env` file

### CS2 Match Script Change

For `/ned match start` to control instance counts, [match.sh](https://github.com/netwarlan/game-deployment-scripts) needs a two-line change:

```bash
# Before:
PRO_INSTANCE_COUNT=1
CASUAL_INSTANCE_COUNT=1

# After:
PRO_INSTANCE_COUNT="${PRO_INSTANCE_COUNT:-1}"
CASUAL_INSTANCE_COUNT="${CASUAL_INSTANCE_COUNT:-1}"
```

## Development

```bash
make build    # build binary (embeds git version/commit/date)
make test     # run tests with race detection
make lint     # run golangci-lint
make docker   # build Docker image
make clean    # remove binary
```

## CI/CD

Pushes to `main` build and publish `ghcr.io/netwarlan/ned:latest` via the shared [action-container-build](https://github.com/netwarlan/action-container-build) workflow.
