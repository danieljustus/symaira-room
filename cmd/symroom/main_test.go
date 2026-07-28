package main

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func buildBinary(t *testing.T) string {
	t.Helper()
	binPath := filepath.Join(t.TempDir(), "symroom")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build test binary: %v, output: %s", err, string(out))
	}
	return binPath
}

func TestCLIUsage(t *testing.T) {
	binPath := buildBinary(t)
	cmd := exec.Command(binPath)
	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected error/exit code 2 when running without arguments, got success")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 2 {
			t.Fatalf("expected exit code 2, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("unexpected error type: %v", err)
	}
}

func TestCLISubcommand(t *testing.T) {
	binPath := buildBinary(t)
	subcommands := []string{
		"init", "identity", "member", "note", "decide", "artifact",
		"log", "verify", "index", "run", "checkpoint", "watch",
		"brain-profile", "doctor", "version", "mcp",
	}
	for _, sub := range subcommands {
		cmd := exec.Command(binPath, sub)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("subcommand %s failed: %v, output: %s", sub, err, string(out))
		}
	}
}
