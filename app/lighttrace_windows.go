//go:build windows && rgb_gui

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"aula-codex-rgb-lab/internal/events"
	"aula-codex-rgb-lab/internal/status"
	"github.com/parsiya/f108-pro/pkg/aula"
)

func taskSignature(tasks []status.Task) string {
	var b strings.Builder
	for _, t := range tasks {
		fmt.Fprintf(&b, "%s/%s/%s;", t.Thread, t.Turn, t.State)
	}
	if len(tasks) == 0 {
		return "idle"
	}
	return b.String()
}

func (a *application) traceLightFrame(phase string, tasks []status.Task, frame aula.LEDFrame, transfer time.Duration) {
	if !events.DiagnosticsEnabled() {
		return
	}
	states := []map[string]any{}
	for _, t := range tasks {
		s := map[string]any{"thread": t.Thread, "turn": t.Turn, "state": t.State, "confirmed": t.Confirmed, "source": t.Source, "event": t.Event}
		s["active_tools"], s["pending_approvals"], s["stale"] = t.ActiveTools, t.PendingApprovals, t.Stale
		if !t.ReceivedAt.IsZero() {
			s["received_to_frame_ms"] = float64(time.Since(t.ReceivedAt).Microseconds()) / 1000
		}
		states = append(states, s)
	}
	functionRow := map[string][3]uint8{}
	for n := 1; n <= 12; n++ {
		name := fmt.Sprintf("f%d", n)
		functionRow[name] = frame[aula.KeyNameToIndex[name]]
	}
	b, err := json.Marshal(map[string]any{"at": time.Now().UTC(), "phase": phase, "usb_success": true, "brightness": a.brightness.Load(), "connections": "flow", "function_row_rgb": functionRow, "transfer_ms": float64(transfer.Microseconds()) / 1000, "tasks": states})
	if err != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(a.dataDir, "light-events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
