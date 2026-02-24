package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellExecutor_Run(t *testing.T) {
	// Create a temporary script that echoes its arguments and environment.
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "testservice")
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		t.Fatal(err)
	}

	script := `#!/bin/bash
echo "command=$1"
echo "NETWAR_ENV=$NETWAR_ENV"
echo "CUSTOM_VAR=$CUSTOM_VAR"
`
	scriptPath := filepath.Join(svcDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	exec := NewShellExecutor(dir, "local")
	result, err := exec.Run(context.Background(), "testservice/test.sh", "down", map[string]string{
		"CUSTOM_VAR": "hello",
	})
	if err != nil {
		t.Fatal(err)
	}

	if result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", result.ExitCode)
	}
	if !strings.Contains(result.Stdout, "command=down") {
		t.Errorf("stdout missing command=down: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "NETWAR_ENV=local") {
		t.Errorf("stdout missing NETWAR_ENV=local: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "CUSTOM_VAR=hello") {
		t.Errorf("stdout missing CUSTOM_VAR=hello: %s", result.Stdout)
	}
}

func TestShellExecutor_WorkingDirectory(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "myservice")
	if err := os.MkdirAll(svcDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Script prints its working directory.
	script := `#!/bin/bash
pwd
`
	if err := os.WriteFile(filepath.Join(svcDir, "svc.sh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	exec := NewShellExecutor(dir, "event")
	result, err := exec.Run(context.Background(), "myservice/svc.sh", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	got := strings.TrimSpace(result.Stdout)
	// Resolve symlinks for comparison (macOS /var → /private/var).
	wantResolved, _ := filepath.EvalSymlinks(svcDir)
	if got != svcDir && got != wantResolved {
		t.Errorf("working dir = %q, want %q", got, svcDir)
	}
}

func TestShellExecutor_NonzeroExit(t *testing.T) {
	dir := t.TempDir()

	script := `#!/bin/bash
echo "failing" >&2
exit 42
`
	if err := os.WriteFile(filepath.Join(dir, "fail.sh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	exec := NewShellExecutor(dir, "event")
	result, err := exec.Run(context.Background(), "fail.sh", "", nil)
	if err != nil {
		t.Fatal(err) // non-zero exit should not be returned as an error
	}

	if result.ExitCode != 42 {
		t.Errorf("exit code = %d, want 42", result.ExitCode)
	}
	if !strings.Contains(result.Stderr, "failing") {
		t.Errorf("stderr missing 'failing': %s", result.Stderr)
	}
}

func TestMatchExecutor_Start_Validation(t *testing.T) {
	exec := NewShellExecutor(t.TempDir(), "event")
	m := NewMatchExecutor(exec, "match.sh", 10, 5)

	_, err := m.Start(context.Background(), 0, 0)
	if err == nil {
		t.Error("expected error for 0/0 counts")
	}

	_, err = m.Start(context.Background(), 11, 0)
	if err == nil {
		t.Error("expected error for pro > max")
	}

	_, err = m.Start(context.Background(), 0, 6)
	if err == nil {
		t.Error("expected error for casual > max")
	}
}

func TestMatchExecutor_Start_EnvVars(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/bash
echo "PRO=$PRO_INSTANCE_COUNT"
echo "CASUAL=$CASUAL_INSTANCE_COUNT"
echo "CMD=$1"
`
	if err := os.WriteFile(filepath.Join(dir, "match.sh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	exec := NewShellExecutor(dir, "event")
	m := NewMatchExecutor(exec, "match.sh", 10, 5)

	result, err := m.Start(context.Background(), 3, 2)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Stdout, "PRO=3") {
		t.Errorf("stdout missing PRO=3: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "CASUAL=2") {
		t.Errorf("stdout missing CASUAL=2: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "CMD=up") {
		t.Errorf("stdout missing CMD=up: %s", result.Stdout)
	}
}

func TestMatchExecutor_Stop_UsesMaxCounts(t *testing.T) {
	dir := t.TempDir()
	script := `#!/bin/bash
echo "PRO=$PRO_INSTANCE_COUNT"
echo "CASUAL=$CASUAL_INSTANCE_COUNT"
echo "CMD=$1"
`
	if err := os.WriteFile(filepath.Join(dir, "match.sh"), []byte(script), 0755); err != nil {
		t.Fatal(err)
	}

	exec := NewShellExecutor(dir, "event")
	m := NewMatchExecutor(exec, "match.sh", 10, 5)

	result, err := m.Stop(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result.Stdout, "PRO=10") {
		t.Errorf("stop should use max pro count: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "CASUAL=5") {
		t.Errorf("stop should use max casual count: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "CMD=down") {
		t.Errorf("stop should pass 'down' command: %s", result.Stdout)
	}
}
