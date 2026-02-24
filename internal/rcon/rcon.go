package rcon

import (
	"context"
	"fmt"
	"time"

	gorcon "github.com/gorcon/rcon"
)

// Client defines the interface for sending RCON commands.
type Client interface {
	Execute(ctx context.Context, address, password, command string) (string, error)
}

// GorconClient implements Client using github.com/gorcon/rcon.
type GorconClient struct {
	timeout time.Duration
}

// NewGorconClient creates a new RCON client with the specified timeout.
func NewGorconClient(timeout time.Duration) *GorconClient {
	return &GorconClient{timeout: timeout}
}

// Execute connects to the server, authenticates, sends the command, and disconnects.
// Each call creates a fresh connection — RCON connections are cheap and game servers
// have limited connection slots.
func (c *GorconClient) Execute(ctx context.Context, address, password, command string) (string, error) {
	conn, err := gorcon.Dial(address, password, gorcon.SetDeadline(c.timeout))
	if err != nil {
		return "", fmt.Errorf("connecting to %s: %w", address, err)
	}
	defer conn.Close()

	response, err := conn.Execute(command)
	if err != nil {
		return "", fmt.Errorf("executing command on %s: %w", address, err)
	}

	return response, nil
}
