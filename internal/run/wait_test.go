package run

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/danieljustus/symaira-room/internal/identity"

	"github.com/danieljustus/symaira-room/internal/event"
	"github.com/danieljustus/symaira-room/internal/journal"
)

func TestWaitApprovedPromptly(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, _ := identity.Generate("owner")

	evReq, _ := Request(tempDir, "Wait Test", "", "", ownerID)
	var bReq struct {
		RunID string `json:"run_id"`
	}
	json.Unmarshal(evReq.Body, &bReq)
	runID := bReq.RunID

	// Approve run asynchronously after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		j := journal.New(filepath.Join(tempDir, "journal"))
		evApp := &event.Event{
			V:      event.CurrentVersion,
			ID:     "ev_app_wait",
			Room:   "rm_test",
			Author: ownerID.MemberID,
			Kind:   event.KindRunApproved,
			Body:   []byte(`{"run_id":"` + runID + `"}`),
		}
		j.PrepareEvent(evApp)
		evApp.Sign(ownerID)
		j.Append(evApp)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	r, err := Wait(ctx, tempDir, runID, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("Wait took longer than 2 seconds")
	}
	if r.State != StateApproved {
		t.Errorf("expected approved state, got %s", r.State)
	}
}

func TestWaitTimeout(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, _ := identity.Generate("owner")

	evReq, _ := Request(tempDir, "Timeout Test", "", "", ownerID)
	var bTimeout struct {
		RunID string `json:"run_id"`
	}
	json.Unmarshal(evReq.Body, &bTimeout)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := Wait(ctx, tempDir, bTimeout.RunID, 20*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}
	if err != ErrWaitTimeout {
		t.Errorf("expected ErrWaitTimeout, got %v", err)
	}
}
