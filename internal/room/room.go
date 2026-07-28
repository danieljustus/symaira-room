package room

import (
	"crypto/rand"

	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/danieljustus/symaira-corekit/fsutil"
	"github.com/danieljustus/symaira-room/internal/event"
	"github.com/danieljustus/symaira-room/internal/identity"
)

var (
	ErrNotEmpty = errors.New("room directory is not empty")
)

type RoomConfig struct {
	SchemaVersion int    `json:"schema_version"`
	ID            string `json:"id"`
	Created       string `json:"created"`
	RootPubkey    string `json:"root_pubkey"`
	RootEvent     string `json:"root_event"`
}

type LocalConfig struct {
	Identity     string `json:"identity"`
	ArtifactRoot string `json:"artifact_root"`
}

func GenerateRoomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return "rm_" + hex.EncodeToString(b)
}

func GenerateEventID() string {
	b := make([]byte, 10)
	rand.Read(b)
	return "ev_" + hex.EncodeToString(b)
}

func Init(dir, name string, id *identity.Identity) (*RoomConfig, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("abs dir: %w", err)
	}

	entries, err := os.ReadDir(absDir)
	if err == nil && len(entries) > 0 {
		return nil, ErrNotEmpty
	}

	if err := fsutil.SafeMkdirAll(absDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir room dir: %w", err)
	}

	roomID := GenerateRoomID()
	eventID := GenerateEventID()
	nowStr := event.FormatTimestamp(time.Now())

	bodyMap := map[string]string{
		"name":       name,
		"public_key": hex.EncodeToString(id.PublicKey),
	}
	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("marshal event body: %w", err)
	}

	ev := &event.Event{
		V:       event.CurrentVersion,
		ID:      eventID,
		Room:    roomID,
		Author:  id.MemberID,
		Seq:     1,
		Prev:    "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Lamport: 1,
		TS:      nowStr,
		Kind:    event.KindRoomCreated,
		Body:    json.RawMessage(bodyBytes),
	}

	if err := ev.Sign(id); err != nil {
		return nil, fmt.Errorf("sign room.created event: %w", err)
	}

	// 1. Write journal/<owner-member-id>.jsonl
	journalDir := filepath.Join(absDir, "journal")
	if err := fsutil.SafeMkdirAll(journalDir, 0755); err != nil {
		return nil, fmt.Errorf("mkdir journal dir: %w", err)
	}
	eventLine, err := ev.MarshalJSONLine()
	if err != nil {
		return nil, fmt.Errorf("marshal event line: %w", err)
	}
	journalFile := filepath.Join(journalDir, id.MemberID+".jsonl")
	if err := fsutil.AtomicWriteFile(journalFile, eventLine, 0644); err != nil {
		return nil, fmt.Errorf("write journal file: %w", err)
	}

	// 2. Write room.toml
	roomTOMLContent := fmt.Sprintf(`schema_version = 1
id = "%s"
created = "%s"
root_pubkey = "ed25519:%s"
root_event = "%s"
`, roomID, nowStr, hex.EncodeToString(id.PublicKey), eventID)

	if err := fsutil.AtomicWriteFile(filepath.Join(absDir, "room.toml"), []byte(roomTOMLContent), 0644); err != nil {
		return nil, fmt.Errorf("write room.toml: %w", err)
	}

	// 3. Write .symroom/local.toml
	dotSymroom := filepath.Join(absDir, ".symroom")
	if err := fsutil.SafeMkdirAll(dotSymroom, 0755); err != nil {
		return nil, fmt.Errorf("mkdir .symroom: %w", err)
	}

	localTOMLContent := fmt.Sprintf(`identity = "%s"
artifact_root = ""
`, id.Name)
	if err := fsutil.AtomicWriteFile(filepath.Join(dotSymroom, "local.toml"), []byte(localTOMLContent), 0644); err != nil {
		return nil, fmt.Errorf("write .symroom/local.toml: %w", err)
	}

	// 4. Write .gitignore in room root
	gitIgnorePath := filepath.Join(absDir, ".gitignore")
	gitIgnoreContent := ".symroom/\n"
	if _, err := os.Stat(gitIgnorePath); os.IsNotExist(err) {
		if err := fsutil.AtomicWriteFile(gitIgnorePath, []byte(gitIgnoreContent), 0644); err != nil {
			return nil, fmt.Errorf("write .gitignore: %w", err)
		}
	}

	return &RoomConfig{
		SchemaVersion: 1,
		ID:            roomID,
		Created:       nowStr,
		RootPubkey:    "ed25519:" + hex.EncodeToString(id.PublicKey),
		RootEvent:     eventID,
	}, nil
}
