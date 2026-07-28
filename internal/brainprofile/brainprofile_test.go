package brainprofile

import (
	"strings"
	"testing"

	"github.com/danieljustus/symaira-room/internal/identity"
	"github.com/danieljustus/symaira-room/internal/room"
)

func TestBrainProfileGenerate(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, _ := identity.Generate("owner")

	if _, err := room.Init(tempDir, "Project Alpha", ownerID); err != nil {
		t.Fatalf("Init room failed: %v", err)
	}

	content, prof, err := Generate(tempDir, ownerID.MemberID)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if prof.Name != "room-project-alpha" {
		t.Errorf("expected profile name room-project-alpha, got %s", prof.Name)
	}

	if !strings.Contains(content, `name = "room-project-alpha"`) {
		t.Errorf("content missing profile name: %s", content)
	}
	if !strings.Contains(content, `member_id = "`+ownerID.MemberID+`"`) {
		t.Errorf("content missing member_id: %s", content)
	}
}
