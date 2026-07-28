package journal

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/danieljustus/symaira-room/internal/event"
)

var (
	ErrChainBroken = errors.New("journal hash chain broken")
	ErrSeqMismatch = errors.New("journal sequence number mismatch")
)

type Journal struct {
	Dir string
	mu  sync.RWMutex
}

func New(dir string) *Journal {
	return &Journal{Dir: dir}
}

func (j *Journal) SegmentPath(author string) string {
	return filepath.Join(j.Dir, author+".jsonl")
}

func ComputeLineHash(line []byte) string {
	h := sha256.Sum256(line)
	return "sha256:" + hex.EncodeToString(h[:])
}

func (j *Journal) Append(ev *event.Event) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if err := os.MkdirAll(j.Dir, 0755); err != nil {
		return fmt.Errorf("mkdir journal dir: %w", err)
	}

	if ev.Lamport == 0 {
		maxL, _ := j.maxLamportUnlocked()
		ev.Lamport = maxL + 1
	}

	segPath := j.SegmentPath(ev.Author)
	lastSeq, lastHash, err := readLastSegmentStats(segPath)
	if err != nil {
		return err
	}

	ev.Seq = lastSeq + 1
	ev.Prev = lastHash

	line, err := ev.MarshalJSONLine()
	if err != nil {
		return fmt.Errorf("marshal event line: %w", err)
	}

	f, err := os.OpenFile(segPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("open segment file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("write segment line: %w", err)
	}
	return nil
}

func readLastSegmentStats(path string) (uint64, string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, "sha256:0000000000000000000000000000000000000000000000000000000000000000", nil
		}
		return 0, "", err
	}
	defer f.Close()

	var lastLine []byte
	var count uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) > 0 {
			count++
			lastLine = append([]byte(nil), line...)
		}
	}

	if count == 0 {
		return 0, "sha256:0000000000000000000000000000000000000000000000000000000000000000", nil
	}

	return count, ComputeLineHash(lastLine), nil
}

func (j *Journal) ReadSegment(author string) ([]*event.Event, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.readSegmentUnlocked(author)
}

func (j *Journal) readSegmentUnlocked(author string) ([]*event.Event, error) {
	segPath := j.SegmentPath(author)
	f, err := os.Open(segPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []*event.Event{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var events []*event.Event
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(strings.TrimSpace(string(line))) == 0 {
			continue
		}
		ev, err := event.UnmarshalJSONLine(line)
		if err != nil {
			return nil, fmt.Errorf("unmarshal line: %w", err)
		}
		events = append(events, ev)
	}

	return events, nil
}

func (j *Journal) VerifyChain(author string) error {
	j.mu.RLock()
	defer j.mu.RUnlock()

	events, err := j.readSegmentUnlocked(author)
	if err != nil {
		return err
	}
	segPath := j.SegmentPath(author)
	f, err := os.Open(segPath)
	if err != nil {
		if os.IsNotExist(err) && len(events) == 0 {
			return nil
		}
		return err
	}
	defer f.Close()

	lines, err := readLines(f)
	if err != nil {
		return err
	}

	var prevHash = "sha256:0000000000000000000000000000000000000000000000000000000000000000"
	var expectedSeq uint64 = 1

	for idx, lineBytes := range lines {
		if idx >= len(events) {
			break
		}
		ev := events[idx]
		if ev.Seq != expectedSeq {
			return fmt.Errorf("%w: author %s expected seq %d, got %d", ErrSeqMismatch, author, expectedSeq, ev.Seq)
		}
		if ev.Prev != prevHash {
			return fmt.Errorf("%w: author %s seq %d expected prev %s, got %s", ErrChainBroken, author, ev.Seq, prevHash, ev.Prev)
		}
		prevHash = ComputeLineHash(lineBytes)
		expectedSeq++
	}
	return nil
}

func (j *Journal) ReadAllSegments() (map[string][]*event.Event, error) {
	j.mu.RLock()
	defer j.mu.RUnlock()

	return j.readAllSegmentsUnlocked()
}

func (j *Journal) readAllSegmentsUnlocked() (map[string][]*event.Event, error) {
	entries, err := os.ReadDir(j.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]*event.Event{}, nil
		}
		return nil, err
	}

	result := make(map[string][]*event.Event)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".jsonl") {
			author := strings.TrimSuffix(entry.Name(), ".jsonl")
			events, err := j.readSegmentUnlocked(author)
			if err != nil {
				return nil, fmt.Errorf("read segment %s: %w", author, err)
			}
			result[author] = events
		}
	}
	return result, nil
}
