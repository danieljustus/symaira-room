package desk_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danieljustus/symaira-room/internal/artifact"
	"github.com/danieljustus/symaira-room/internal/desk"
	"github.com/danieljustus/symaira-room/internal/event"
	"github.com/danieljustus/symaira-room/internal/identity"
	"github.com/danieljustus/symaira-room/internal/journal"
)

func TestWatchStreamOnlyLinkedArtifactsProduceEvents(t *testing.T) {
	tempDir := t.TempDir()
	ownerID, err := identity.Generate("owner")
	if err != nil {
		t.Fatalf("generate identity: %v", err)
	}

	linkedPath := filepath.Join(tempDir, "linked.txt")
	if err := os.WriteFile(linkedPath, []byte("original content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	unlinkedPath := filepath.Join(tempDir, "unlinked.txt")
	if err := os.WriteFile(unlinkedPath, []byte("noise content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Link only linkedPath
	_, err = artifact.Link(tempDir, tempDir, linkedPath, "Linked Doc", ownerID)
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}

	// Modify linked file
	if err := os.WriteFile(linkedPath, []byte("updated content"), 0644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Simulate NDJSON stream containing both linked and unlinked file events
	ndjsonInput := `{"event":"file_changed","path":"` + unlinkedPath + `"}
{"event":"file_changed","path":"` + linkedPath + `"}
`

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	buf := bytes.NewBufferString(ndjsonInput)
	handler := func(item *desk.EventStreamItem) error {
		return artifact.HandleDeskEvent(tempDir, tempDir, item, ownerID)
	}
	if err := desk.WatchStream(ctx, buf, handler); err != nil {
		t.Fatalf("WatchStream error: %v", err)
	}

	j := journal.New(filepath.Join(tempDir, "journal"))
	merged, err := j.MergeAll()
	if err != nil {
		t.Fatalf("MergeAll failed: %v", err)
	}

	var changedEvents []*event.Event
	for _, ev := range merged {
		if ev.Kind == event.KindArtifactChanged {
			changedEvents = append(changedEvents, ev)
		}
	}

	// Exactly 1 artifact.changed event produced (for linked file, 0 for unlinked noise)
	if len(changedEvents) != 1 {
		t.Fatalf("expected 1 artifact.changed event for linked file, got %d", len(changedEvents))
	}
}
