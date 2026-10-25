//go:build windows

package events

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type cursor struct {
	offset     int64
	pending    string
	discarding bool
	info       os.FileInfo
	thread     string
}
type Tailer struct {
	base    string
	cursors map[string]*cursor
	started time.Time
	lookups map[string]time.Time
}

func NewTailer(base string) *Tailer {
	return &Tailer{base: base, cursors: map[string]*cursor{}, started: time.Now()}
}

// WatchThread follows an active hook session even when its creation-date
// directory is older than the normal discovery window. File names are only a
// lookup hint: the session metadata must match before the file is accepted.
// Call from the tailer's owner goroutine, before Poll.
func (t *Tailer) WatchThread(thread string) {
	thread = strings.SplitN(thread, "#subagent:", 2)[0]
	if thread == "" {
		return
	}
	for _, c := range t.cursors {
		if c.thread == thread {
			return
		}
	}
	if t.lookups == nil {
		t.lookups = map[string]time.Time{}
	}
	now := time.Now()
	if last := t.lookups[thread]; !last.IsZero() && now.Sub(last) < 30*time.Second {
		return
	}
	t.lookups[thread] = now
	_ = filepath.WalkDir(t.base, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") || !strings.Contains(entry.Name(), thread) {
			return nil
		}
		id, ok := desktopSession(path)
		if !ok || id != thread {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		if t.cursors == nil {
			t.cursors = map[string]*cursor{}
		}
		// The runtime filters pre-start events and the engine matches the live
		// turn. Reading from zero also recovers a terminal already on disk.
		t.cursors[path] = &cursor{info: info, thread: thread}
		delete(t.lookups, thread)
		return filepath.SkipAll
	})
}
func desktopSession(path string) (string, bool) {
	f, err := os.Open(path)
	if err != nil {
		return "", false
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 4096), 1<<20)
	if !s.Scan() {
		return "", false
	}
	var root struct {
		Type    string `json:"type"`
		Payload struct {
			ID         string `json:"id"`
			Originator string `json:"originator"`
		} `json:"payload"`
	}
	if json.Unmarshal(s.Bytes(), &root) != nil || root.Type != "session_meta" {
		return "", false
	}
	if root.Payload.ID == "" || (root.Payload.Originator != "codex_work_desktop" && root.Payload.Originator != "codex_desktop" && root.Payload.Originator != "Codex Desktop" && root.Payload.Originator != "codex_exec" && root.Payload.Originator != "codex_cli_rs") {
		return "", false
	}
	return root.Payload.ID, true
}
func (t *Tailer) Poll(emit func([]byte, string, string)) error {
	if t.cursors == nil {
		t.cursors = map[string]*cursor{}
	}
	for days := 0; days < 8; days++ {
		d := time.Now().AddDate(0, 0, -days)
		dir := filepath.Join(t.base, d.Format("2006"), d.Format("01"), d.Format("02"))
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			if t.cursors[path] != nil {
				continue
			}
			info, err := os.Stat(path)
			if err != nil {
				continue
			}
			thread, ok := desktopSession(path)
			if !ok {
				continue
			}
			offset := info.Size()
			if info.ModTime().After(t.started) {
				offset = 0
			}
			t.cursors[path] = &cursor{offset: offset, info: info, thread: thread}
		}
	}
	// Poll all registered cursors, including old-date sessions enrolled by hooks.
	for path, c := range t.cursors {
		info, err := os.Stat(path)
		if os.IsNotExist(err) {
			delete(t.cursors, path)
			continue
		}
		if err != nil {
			continue
		}
		t.pollFile(path, c, info, emit)
	}
	for thread, at := range t.lookups {
		if time.Since(at) > 5*time.Minute {
			delete(t.lookups, thread)
		}
	}
	return nil
}

func (t *Tailer) pollFile(path string, c *cursor, info os.FileInfo, emit func([]byte, string, string)) {
	if !os.SameFile(c.info, info) || info.Size() < c.offset {
		c.offset = 0
		c.pending = ""
		c.discarding = false
		c.info = info
	}
	if info.Size() <= c.offset {
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	// Old chats can exceed 100MB. Stream a fixed-size snapshot instead of
	// allocating the entire unread transcript, and bound an incomplete line.
	reader := bufio.NewReaderSize(io.NewSectionReader(f, c.offset, info.Size()-c.offset), 64<<10)
	for {
		fragment, err := reader.ReadSlice('\n')
		c.offset += int64(len(fragment))
		if !c.discarding {
			if len(c.pending)+len(fragment) > 8<<20 {
				c.pending = ""
				c.discarding = true
			} else {
				c.pending += string(fragment)
			}
		}
		if len(fragment) > 0 && fragment[len(fragment)-1] == '\n' {
			if !c.discarding {
				if line := strings.TrimSpace(c.pending); line != "" {
					emit([]byte(line), path, c.thread)
				}
			}
			c.pending = ""
			c.discarding = false
		}
		if err != nil && err != bufio.ErrBufferFull {
			return
		}
	}
}
