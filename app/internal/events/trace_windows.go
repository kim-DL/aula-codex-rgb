//go:build windows

package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"aula-codex-rgb-lab/internal/status"
)

var diagnostics = os.Getenv("AULA_RGB_DIAGNOSTICS") == "1"

func DiagnosticsEnabled() bool { return diagnostics }

// AppendMetadata records neither message bodies, commands, outputs, nor diffs.
func AppendMetadata(ev status.Event, accepted bool) {
	if !diagnostics {
		return
	}
	dir := filepath.Join(os.Getenv("LOCALAPPDATA"), "AulaCodexRgb")
	b, err := json.Marshal(map[string]any{
		"thread": ev.Thread, "turn": ev.Turn, "state": ev.Type, "source": ev.Source,
		"tool": ev.Tool, "error": ev.Error, "accepted": accepted, "recorded_at": ev.At,
		"call_id":           ev.Call,
		"temporary_session": ev.Temporary,
		"received_at":       ev.ReceivedAt, "file_extension": filepath.Ext(ev.Path),
	})
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(time.Now().UTC().Format(time.RFC3339Nano) + " " + string(b) + "\n")
}
