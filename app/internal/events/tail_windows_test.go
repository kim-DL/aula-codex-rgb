//go:build windows

package events

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTailerSkipsHistoryAndWaitsForCompleteLines(t *testing.T) {
	now := time.Now()
	dir := filepath.Join(t.TempDir(), now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "rollout-2026-10-14T00-00-00-00000000-0000-0000-0000-000000000001.jsonl")
	initial := "{\"type\":\"session_meta\",\"payload\":{\"id\":\"desktop-session-1\",\"originator\":\"codex_work_desktop\"}}\nold-event\n"
	if err := os.WriteFile(path, []byte(initial), 0600); err != nil {
		t.Fatal(err)
	}
	tailer := NewTailer(filepath.Dir(filepath.Dir(filepath.Dir(dir))))
	var got [][]byte
	gotThread := ""
	emit := func(line []byte, _ string, thread string) {
		got = append(got, append([]byte(nil), line...))
		gotThread = thread
	}
	if err := tailer.Poll(emit); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatal("existing history was replayed at startup")
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("{\"type\":\"event_msg\",\"payload\":")
	_ = f.Close()
	if err := tailer.Poll(emit); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatal("partial JSONL line was emitted")
	}
	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("{\"type\":\"task_complete\"}}\n")
	_ = f.Close()
	if err := tailer.Poll(emit); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("completed line count=%d", len(got))
	}
	if gotThread != "desktop-session-1" {
		t.Fatalf("session id=%q", gotThread)
	}
}
