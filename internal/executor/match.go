package executor

import (
	"context"
	"fmt"
	"strconv"
)

// MatchExecutor handles CS2 match server lifecycle.
// It calls cs2.sh with: match up --count N / match down --count N
type MatchExecutor struct {
	executor      *ShellExecutor
	scriptPath    string
	maxInstances  int
}

// NewMatchExecutor creates a MatchExecutor.
// scriptPath is relative to scriptsDir (e.g., "cs2/cs2.sh").
func NewMatchExecutor(executor *ShellExecutor, scriptPath string, maxInstances int) *MatchExecutor {
	return &MatchExecutor{
		executor:     executor,
		scriptPath:   scriptPath,
		maxInstances: maxInstances,
	}
}

// Start spins up the specified number of match instances.
// Calls: cs2.sh match up --count N
func (m *MatchExecutor) Start(ctx context.Context, count int) (*Result, error) {
	if count <= 0 || count > m.maxInstances {
		return nil, fmt.Errorf("count must be 1-%d, got %d", m.maxInstances, count)
	}

	return m.executor.Run(ctx, m.scriptPath, "match up --count "+strconv.Itoa(count), nil)
}

// Stop tears down all match instances using max count to ensure all are caught.
// Calls: cs2.sh match down --count N
func (m *MatchExecutor) Stop(ctx context.Context) (*Result, error) {
	return m.executor.Run(ctx, m.scriptPath, "match down --count "+strconv.Itoa(m.maxInstances), nil)
}
