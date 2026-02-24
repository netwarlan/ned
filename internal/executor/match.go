package executor

import (
	"context"
	"fmt"
	"strconv"
)

// MatchExecutor handles CS2 match server lifecycle.
type MatchExecutor struct {
	executor   *ShellExecutor
	scriptPath string
	maxPro     int
	maxCasual  int
}

// NewMatchExecutor creates a MatchExecutor.
// scriptPath is relative to scriptsDir (e.g., "cs2/match/match.sh").
func NewMatchExecutor(executor *ShellExecutor, scriptPath string, maxPro, maxCasual int) *MatchExecutor {
	return &MatchExecutor{
		executor:   executor,
		scriptPath: scriptPath,
		maxPro:     maxPro,
		maxCasual:  maxCasual,
	}
}

// Start spins up the specified number of pro and casual match instances.
func (m *MatchExecutor) Start(ctx context.Context, proCount, casualCount int) (*Result, error) {
	if proCount < 0 || proCount > m.maxPro {
		return nil, fmt.Errorf("pro count must be 0-%d, got %d", m.maxPro, proCount)
	}
	if casualCount < 0 || casualCount > m.maxCasual {
		return nil, fmt.Errorf("casual count must be 0-%d, got %d", m.maxCasual, casualCount)
	}
	if proCount == 0 && casualCount == 0 {
		return nil, fmt.Errorf("at least one of pro or casual count must be > 0")
	}

	env := map[string]string{
		"PRO_INSTANCE_COUNT":    strconv.Itoa(proCount),
		"CASUAL_INSTANCE_COUNT": strconv.Itoa(casualCount),
	}

	return m.executor.Run(ctx, m.scriptPath, "up", env)
}

// Stop tears down all match instances using max counts to ensure all are caught.
func (m *MatchExecutor) Stop(ctx context.Context) (*Result, error) {
	env := map[string]string{
		"PRO_INSTANCE_COUNT":    strconv.Itoa(m.maxPro),
		"CASUAL_INSTANCE_COUNT": strconv.Itoa(m.maxCasual),
	}

	return m.executor.Run(ctx, m.scriptPath, "down", env)
}
