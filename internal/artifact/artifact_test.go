package artifact

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/danieljustus/symaira-room/internal/identity"
)

func TestArtifactLinkUnlinkList(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, err := identity.Generate("owner")
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	docPath := filepath.Join(tempDir, "doc.txt")
	if err := os.WriteFile(docPath, []byte("hello artifact"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// 1. Link artifact inside root
	evLink, err := Link(tempDir, tempDir, docPath, "Document One", ownerID)
	if err != nil {
		t.Fatalf("Link artifact failed: %v", err)
	}
	if evLink == nil {
		t.Fatalf("expected non-nil event")
	}

	// 2. List artifacts -> status ok
	list, err := List(tempDir, tempDir)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(list))
	}
	if list[0].Status != "ok" {
		t.Errorf("expected status ok, got %s", list[0].Status)
	}

	// 3. Modify artifact file -> status modified
	if err := os.WriteFile(docPath, []byte("modified content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	listMod, _ := List(tempDir, tempDir)
	if len(listMod) != 1 || listMod[0].Status != "modified" {
		t.Errorf("expected status modified, got %s", listMod[0].Status)
	}

	// 4. Unlink artifact
	artID := list[0].ID
	_, err = Unlink(tempDir, artID, ownerID)
	if err != nil {
		t.Fatalf("Unlink failed: %v", err)
	}

	listAfter, _ := List(tempDir, tempDir)
	if len(listAfter) != 0 {
		t.Errorf("expected 0 artifacts after unlink, got %d", len(listAfter))
	}
}

func TestArtifactOutsideRootRefused(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, _ := identity.Generate("owner")

	outsidePath := filepath.Join(os.TempDir(), "outside.txt")
	if err := os.WriteFile(outsidePath, []byte("outside"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	defer func() { _ = os.Remove(outsidePath) }()

	_, err := Link(tempDir, tempDir, outsidePath, "Outside Doc", ownerID)
	if err == nil {
		t.Fatalf("expected error for path outside root, got nil")
	}
	if err != ErrOutsideRoot {
		t.Errorf("expected ErrOutsideRoot, got %v", err)
	}
}
