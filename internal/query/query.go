package query

import (
	"context"
	"fmt"
	"time"

	"github.com/rumblefrog/go-a2s"
)

// ServerStatus represents the queried state of a game server.
type ServerStatus struct {
	Online     bool
	Name       string
	Map        string
	Players    int
	MaxPlayers int
	Bots       int
	Latency    time.Duration
}

// PlayerInfo represents a single connected player.
type PlayerInfo struct {
	Name     string
	Score    int
	Duration time.Duration
}

// Querier defines the interface for querying game server status.
type Querier interface {
	QueryStatus(ctx context.Context, address string) (*ServerStatus, error)
	QueryPlayers(ctx context.Context, address string) ([]PlayerInfo, error)
}

// A2SQuerier implements Querier using the A2S protocol.
type A2SQuerier struct {
	timeout time.Duration
}

// NewA2SQuerier creates a new A2S querier with the specified timeout.
func NewA2SQuerier(timeout time.Duration) *A2SQuerier {
	return &A2SQuerier{timeout: timeout}
}

func (q *A2SQuerier) QueryStatus(ctx context.Context, address string) (*ServerStatus, error) {
	client, err := a2s.NewClient(address, a2s.TimeoutOption(q.timeout))
	if err != nil {
		return &ServerStatus{Online: false}, fmt.Errorf("creating A2S client: %w", err)
	}
	defer client.Close()

	start := time.Now()
	info, err := client.QueryInfo()
	latency := time.Since(start)

	if err != nil {
		return &ServerStatus{Online: false}, nil
	}

	return &ServerStatus{
		Online:     true,
		Name:       info.Name,
		Map:        info.Map,
		Players:    int(info.Players),
		MaxPlayers: int(info.MaxPlayers),
		Bots:       int(info.Bots),
		Latency:    latency,
	}, nil
}

func (q *A2SQuerier) QueryPlayers(ctx context.Context, address string) ([]PlayerInfo, error) {
	client, err := a2s.NewClient(address, a2s.TimeoutOption(q.timeout))
	if err != nil {
		return nil, fmt.Errorf("creating A2S client: %w", err)
	}
	defer client.Close()

	players, err := client.QueryPlayer()
	if err != nil {
		return nil, fmt.Errorf("querying players: %w", err)
	}

	result := make([]PlayerInfo, 0, len(players.Players))
	for _, p := range players.Players {
		result = append(result, PlayerInfo{
			Name:     p.Name,
			Score:    int(p.Score),
			Duration: time.Duration(p.Duration) * time.Second,
		})
	}
	return result, nil
}
