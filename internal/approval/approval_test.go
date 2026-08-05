package approval

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/danieljustus/symaira-room/internal/event"
	"github.com/danieljustus/symaira-room/internal/identity"
	"github.com/danieljustus/symaira-room/internal/journal"
	"github.com/danieljustus/symaira-room/internal/members"
	"github.com/danieljustus/symaira-room/internal/room"
	"github.com/danieljustus/symaira-room/internal/run"
)

func TestApproveAndDenyWithScopeAndTTL(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, err := identity.Generate("owner")
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	if _, err := room.Init(tempDir, "Test Room", ownerID); err != nil {
		t.Fatalf("Init room failed: %v", err)
	}

	// 1. Request a run
	evReq, err := run.Request(tempDir, "Deploy Service", "deploy.md", "shell", ownerID)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	var bReq struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(evReq.Body, &bReq); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}
	runID := bReq.RunID

	// 2. Approve run with scope and TTL
	_, err = Approve(tempDir, runID, "deploy:staging", 30*time.Minute, ownerID)
	if err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	rApp, err := run.Get(tempDir, runID)
	if err != nil {
		t.Fatalf("Get run after approve failed: %v", err)
	}
	if rApp.State != run.StateApproved {
		t.Errorf("expected state approved, got %s", rApp.State)
	}
	if rApp.Scope != "deploy:staging" {
		t.Errorf("expected scope deploy:staging, got %s", rApp.Scope)
	}
}

func TestAgentApprovalForbidden(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, _ := identity.Generate("owner")
	if _, err := room.Init(tempDir, "Test Room", ownerID); err != nil {
		t.Fatalf("Init room failed: %v", err)
	}

	agentID, _ := identity.Generate("bot_worker")

	// Add agent to room with role agent
	agentBody, _ := json.Marshal(map[string]string{
		"id":         agentID.MemberID,
		"public_key": hex.EncodeToString(agentID.PublicKey),
		"role":       string(members.RoleAgent),
		"kind":       string(members.KindAgent),
	})
	j := journal.New(filepath.Join(tempDir, "journal"))
	evAdd := &event.Event{
		V:      event.CurrentVersion,
		ID:     "ev_add_agent",
		Room:   "rm_test",
		Author: ownerID.MemberID,
		Kind:   event.KindMemberAdded,
		Body:   json.RawMessage(agentBody),
	}
	if err := j.PrepareEvent(evAdd); err != nil {
		t.Fatalf("PrepareEvent failed: %v", err)
	}
	if err := evAdd.Sign(ownerID); err != nil {
		t.Fatalf("Sign failed: %v", err)
	}
	if err := j.Append(evAdd); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	evReq, _ := run.Request(tempDir, "Bot Run", "", "", ownerID)
	var bBot struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(evReq.Body, &bBot); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	// Agent attempts to approve -> refused
	_, err := Approve(tempDir, bBot.RunID, "all", 10*time.Minute, agentID)
	if err == nil {
		t.Fatalf("expected ErrAgentApprovalForbidden, got nil")
	}
	if err != ErrAgentApprovalForbidden {
		t.Errorf("expected ErrAgentApprovalForbidden, got %v", err)
	}
}

func TestExpiredApprovalBlocksStart(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, _ := identity.Generate("owner")
	if _, err := room.Init(tempDir, "Test Room", ownerID); err != nil {
		t.Fatalf("Init room failed: %v", err)
	}

	evReq, _ := run.Request(tempDir, "Fast Expire Run", "", "", ownerID)
	var bFast struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(evReq.Body, &bFast); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	// Approve with negative TTL -> immediately expired
	if _, err := Approve(tempDir, bFast.RunID, "all", -1*time.Minute, ownerID); err != nil {
		t.Fatalf("Approve failed: %v", err)
	}

	_, err := run.Start(tempDir, bFast.RunID, ownerID)
	if err == nil {
		t.Fatalf("expected error starting expired run, got nil")
	}
}
