package room

import (
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/danieljustus/symaira-room/internal/event"
	"github.com/danieljustus/symaira-room/internal/identity"
)

func TestInitRoom(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	id, err := identity.Generate("founder")
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	roomDir := filepath.Join(tempDir, "my-room")
	cfg, err := Init(roomDir, "Test Room", id)
	if err != nil {
		t.Fatalf("failed to init room: %v", err)
	}

	if !strings.HasPrefix(cfg.ID, "rm_") {
		t.Errorf("expected room ID prefix rm_, got %s", cfg.ID)
	}

	// Verify room.toml exists
	roomTomlPath := filepath.Join(roomDir, "room.toml")
	if _, err := os.Stat(roomTomlPath); err != nil {
		t.Errorf("expected room.toml to exist: %v", err)
	}

	// Verify .gitignore excludes .symroom/
	gitIgnorePath := filepath.Join(roomDir, ".gitignore")
	data, err := os.ReadFile(gitIgnorePath)
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".symroom/") {
		t.Errorf("expected .gitignore to contain .symroom/")
	}

	// Verify journal/<owner-id>.jsonl exists and room.created verifies against root_pubkey
	journalPath := filepath.Join(roomDir, "journal", id.MemberID+".jsonl")
	jData, err := os.ReadFile(journalPath)
	if err != nil {
		t.Fatalf("failed to read journal file: %v", err)
	}

	ev, err := event.UnmarshalJSONLine(jData)
	if err != nil {
		t.Fatalf("failed to unmarshal room.created event: %v", err)
	}

	if ev.Kind != event.KindRoomCreated {
		t.Errorf("expected event kind room.created, got %s", ev.Kind)
	}

	rawRootPubkey := strings.TrimPrefix(cfg.RootPubkey, "ed25519:")
	pubBytes, err := hex.DecodeString(rawRootPubkey)
	if err != nil {
		t.Fatalf("invalid root pubkey hex: %v", err)
	}

	if err := ev.VerifySignature(ed25519.PublicKey(pubBytes)); err != nil {
		t.Errorf("room.created failed verification against root_pubkey: %v", err)
	}
}

func TestInitRefusesNonEmptyDir(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	id, err := identity.Generate("founder")
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	roomDir := filepath.Join(tempDir, "existing-room")
	if err := os.MkdirAll(roomDir, 0755); err != nil {
		t.Fatalf("failed to mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(roomDir, "some-file.txt"), []byte("data"), 0644); err != nil {
		t.Fatalf("failed to write dummy file: %v", err)
	}

	_, err = Init(roomDir, "Test Room", id)
	if err == nil {
		t.Fatalf("expected error initializing non-empty dir, got nil")
	}
	if err != ErrNotEmpty {
		t.Errorf("expected ErrNotEmpty, got %v", err)
	}
}
