//go:build windows

package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aula-codex-rgb-lab/internal/status"
)

func oldSessionFile(t *testing.T, base, name, thread string) string {
	t.Helper()
	d := time.Now().AddDate(0, 0, -30)
	dir := filepath.Join(base, d.Format("2006"), d.Format("01"), d.Format("02"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-"+name+".jsonl")
	meta, _ := json.Marshal(map[string]any{"type": "session_meta", "payload": map[string]string{"id": thread, "originator": "Codex Desktop"}})
	if err := os.WriteFile(path, append(meta, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWatchOldActiveSessionRecoversAlreadyWrittenCompletion(t *testing.T) {
	base := t.TempDir()
	path := oldSessionFile(t, base, "old-thread", "old-thread")
	_ = oldSessionFile(t, base, "unrelated", "unrelated")
	now := time.Now()
	completeAt := now.Add(-time.Minute)
	raw, _ := json.Marshal(map[string]any{"timestamp": completeAt.Format(time.RFC3339Nano), "type": "event_msg", "payload": map[string]string{"type": "task_complete", "turn_id": "real-turn"}})
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write(append(raw, '\n'))
	_ = f.Close()
	// A long-lived/open transcript can retain an old filesystem write time.
	old := now.AddDate(0, 0, -30)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	e := status.New()
	e.Apply(status.Event{Thread: "old-thread", Turn: "real-turn", Type: "UserPromptSubmit", At: now.Add(-2 * time.Minute)})
	e.Apply(status.Event{Thread: "old-thread", Turn: "real-turn", Call: "unfinished", Type: "PreToolUse", At: now.Add(-90 * time.Second)})
	e.Apply(status.Event{Thread: "old-thread", Turn: "real-turn", Type: "Stop", At: now.Add(-50 * time.Second)})
	tail := NewTailer(base)
	emit := func(raw []byte, path, thread string) {
		if v, ok := DecodeForSession(raw, path, thread); ok {
			e.Apply(v)
		}
	}
	_ = tail.Poll(emit)
	if e.Snapshot(now)[0].State != status.Tool {
		t.Fatal("fixture did not reproduce recent-directory exclusion")
	}
	tail.WatchThread("old-thread")
	_ = tail.Poll(emit)
	tasks := e.Snapshot(now)
	if len(tasks) != 1 || tasks[0].State != status.Complete || tasks[0].ActiveTools != 0 {
		t.Fatalf("old session remains busy: %+v", tasks)
	}
	if len(tail.cursors) != 1 {
		t.Fatal("unrelated history was enrolled")
	}
	if len(e.Snapshot(now.Add(10*time.Second))) != 0 {
		t.Fatal("completed old task did not expire to idle")
	}
}

func TestOldSessionContinuesTailingPartialAppend(t *testing.T) {
	base := t.TempDir()
	path := oldSessionFile(t, base, "old-active", "old-active")
	tail := NewTailer(base)
	tail.WatchThread("old-active#subagent:child")
	got := 0
	emit := func(raw []byte, path, thread string) {
		if _, ok := DecodeForSession(raw, path, thread); ok {
			got++
		}
	}
	_ = tail.Poll(emit)
	for n, chunk := range []string{`{"type":"event_msg","payload":`, `{"type":"task_complete","turn_id":"t"}}` + "\n"} {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.WriteString(chunk)
		_ = f.Close()
		_ = tail.Poll(emit)
		if got != n {
			t.Fatalf("partial append step %d: got %d", n, got)
		}
	}
}

func TestWatchThreadValidatesSessionMetadata(t *testing.T) {
	base := t.TempDir()
	oldSessionFile(t, base, "wanted-but-wrong-metadata", "other")
	tail := NewTailer(base)
	tail.WatchThread("wanted")
	if len(tail.cursors) != 0 {
		t.Fatal("filename alone admitted the wrong session")
	}
}

func TestOversizedIncompleteLineDoesNotHideNextCompletion(t *testing.T) {
	base := t.TempDir()
	path := oldSessionFile(t, base, "large-active", "large-active")
	tail := NewTailer(base)
	tail.WatchThread("large-active")
	_ = tail.Poll(func([]byte, string, string) {})
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString(strings.Repeat("x", 9<<20))
	_ = f.Close()
	got := 0
	emit := func(raw []byte, path, thread string) {
		if _, ok := DecodeForSession(raw, path, thread); ok {
			got++
		}
	}
	_ = tail.Poll(emit)
	for _, c := range tail.cursors {
		if len(c.pending) > 8<<20 || !c.discarding {
			t.Fatal("incomplete line is unbounded")
		}
	}
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("\n" + `{"type":"event_msg","payload":{"type":"task_complete","turn_id":"t"}}` + "\n")
	_ = f.Close()
	_ = tail.Poll(emit)
	if got != 1 {
		t.Fatal("next valid completion was lost")
	}
}
