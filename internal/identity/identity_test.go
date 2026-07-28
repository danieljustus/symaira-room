package identity

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAndSaveLoad(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)

	id, err := Generate("alice")
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	if !strings.HasPrefix(id.MemberID, "mem_") {
		t.Errorf("expected member_id prefix mem_, got %s", id.MemberID)
	}

	if err := Save(id); err != nil {
		t.Fatalf("failed to save identity: %v", err)
	}

	// Assert key file permissions 0600
	filePath := filepath.Join(IdentitiesDir(), "alice.json")
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat identity file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected file mode 0600, got %o", info.Mode().Perm())
	}

	// Load back
	loaded, err := Load("alice")
	if err != nil {
		t.Fatalf("failed to load identity: %v", err)
	}

	if loaded.Name != id.Name || loaded.MemberID != id.MemberID {
		t.Errorf("loaded identity mismatched: got %+v, want %+v", loaded, id)
	}

	if !bytes.Equal(loaded.PublicKey, id.PublicKey) {
		t.Errorf("public key mismatch")
	}
	if !bytes.Equal(loaded.PrivateKey, id.PrivateKey) {
		t.Errorf("private key mismatch")
	}
}

func TestSignVerifyRoundTrip(t *testing.T) {
	id, err := Generate("bob")
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	msg := []byte("hello symaira-room")
	sig := Sign(id.PrivateKey, msg)

	if !Verify(id.PublicKey, msg, sig) {
		t.Errorf("signature verification failed")
	}

	if Verify(id.PublicKey, []byte("tampered msg"), sig) {
		t.Errorf("expected tampered msg verification to fail")
	}
}

func TestNoPrivateKeyLogging(t *testing.T) {
	id, err := Generate("charlie")
	if err != nil {
		t.Fatalf("failed to generate identity: %v", err)
	}

	// Convert Identity struct to string or JSON (public representation)
	str := id.MemberID + id.Name
	privHex := hex.EncodeToString(id.PrivateKey)

	if strings.Contains(str, privHex) {
		t.Fatalf("private key leaked in string representation")
	}
}

func TestSymvaultFallbackToFile(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tempDir)
	t.Setenv("PATH", tempDir) // Empty path so symvault is absent

	id, err := Generate("dave")
	if err != nil {
		t.Fatalf("failed to generate: %v", err)
	}
	if err := Save(id); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := Load("dave")
	if err != nil {
		t.Fatalf("expected fallback to file without error, got %v", err)
	}

	if loaded.MemberID != id.MemberID {
		t.Errorf("expected member ID %s, got %s", id.MemberID, loaded.MemberID)
	}
}
