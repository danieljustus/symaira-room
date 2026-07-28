package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danieljustus/symaira-corekit/configkit"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Approval.DefaultTTL != "30m" {
		t.Errorf("expected default_ttl 30m, got %s", cfg.Approval.DefaultTTL)
	}
	if !cfg.Updatecheck.Enabled {
		t.Errorf("expected updatecheck.enabled true, got false")
	}
}

func TestEnvOverrides(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		envVal string
		check  func(*testing.T, *Config)
	}{
		{
			name:   "override default_identity",
			envKey: "SYMROOM_DEFAULT_IDENTITY",
			envVal: "agent-alice",
			check: func(t *testing.T, cfg *Config) {
				if cfg.DefaultIdentity != "agent-alice" {
					t.Errorf("expected agent-alice, got %s", cfg.DefaultIdentity)
				}
			},
		},
		{
			name:   "override approval default_ttl",
			envKey: "SYMROOM_APPROVAL_DEFAULT_TTL",
			envVal: "1h",
			check: func(t *testing.T, cfg *Config) {
				if cfg.Approval.DefaultTTL != "1h" {
					t.Errorf("expected 1h, got %s", cfg.Approval.DefaultTTL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)
			loader := NewLoader()
			loader.ResetCache()
			cfg, err := loader.Load()
			if err != nil {
				t.Fatalf("unexpected error loading config: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestInvalidTOML(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)
	t.Setenv("XDG_CONFIG_HOME", tempDir)

	symroomDir := filepath.Join(tempDir, ".config", "symroom")
	if err := os.MkdirAll(symroomDir, 0755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	invalidFile := filepath.Join(symroomDir, "config.toml")
	if err := os.WriteFile(invalidFile, []byte("[approval]\ndefault_ttl = [invalid toml syntax"), 0644); err != nil {
		t.Fatalf("failed to write invalid toml: %v", err)
	}

	t.Logf("DefaultPath: %s, file exists: %v", configkit.DefaultPath("symroom"), func() bool {
		_, err := os.Stat(configkit.DefaultPath("symroom"))
		return err == nil
	}())

	loader := NewLoader()
	loader.ResetCache()
	_, err := loader.Load()
	if err == nil {
		t.Fatalf("expected error for invalid TOML at %s, got nil", invalidFile)
	}
}
