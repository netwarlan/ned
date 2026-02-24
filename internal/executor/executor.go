package executor

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Result holds the output of a script execution.
type Result struct {
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// Executor defines the interface for running service management scripts.
type Executor interface {
	Run(ctx context.Context, scriptPath, command string, env map[string]string) (*Result, error)
}

// ShellExecutor implements Executor by shelling out to bash.
type ShellExecutor struct {
	scriptsDir  string
	environment string
	timeout     time.Duration
}

// NewShellExecutor creates a new ShellExecutor.
// scriptsDir is the absolute path to the game-deployment-scripts directory.
// environment is "event" or "local".
func NewShellExecutor(scriptsDir, environment string) *ShellExecutor {
	return &ShellExecutor{
		scriptsDir:  scriptsDir,
		environment: environment,
		timeout:     120 * time.Second,
	}
}

// Run executes a service script with the given command.
// scriptPath is relative to scriptsDir (e.g., "tf2/tf2.sh").
// command is "up", "down", "restart", or "update".
// env is additional environment variables to set.
func (e *ShellExecutor) Run(ctx context.Context, scriptPath, command string, env map[string]string) (*Result, error) {
	fullPath := filepath.Join(e.scriptsDir, scriptPath)
	workDir := filepath.Dir(fullPath)
	scriptName := filepath.Base(scriptPath)

	// Use a shorter timeout for "up" commands since the shell scripts
	// tail docker compose logs indefinitely after starting.
	timeout := e.timeout
	if command == "up" || command == "start" || command == "u" {
		timeout = 30 * time.Second
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", scriptName, command)
	cmd.Dir = workDir

	// Build environment: inherit current env, add NETWAR_ENV, add extras.
	cmd.Env = append(os.Environ(), "NETWAR_ENV="+e.environment)
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			// Timeout is expected for "up" commands that tail logs.
			// Return what we captured so far as a success.
			if command == "up" || command == "start" || command == "u" {
				return result, nil
			}
			return result, fmt.Errorf("command timed out after %s", timeout)
		} else {
			return result, fmt.Errorf("executing script: %w", err)
		}
	}

	return result, nil
}
