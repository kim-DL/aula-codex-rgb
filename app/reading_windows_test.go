//go:build windows && !rgb_gui

package main

import (
	"aula-codex-rgb-lab/internal/events"
	"aula-codex-rgb-lab/internal/status"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadHintSurvivesRelayWithoutCommandOrDocumentContent(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"hook_event_name": "PreToolUse", "session_id": "s", "tool_use_id": "c", "tool_name": "exec", "tool_input": map[string]any{"code": `text((await tools.exec_command({cmd:"Get-Content 'PRIVATE-PATH.md'"})).output)`}, "prompt": "PRIVATE-PROMPT", "tool_response": "PRIVATE-CONTENT"})
	wire, ok, err := hookMetadata(raw)
	if err != nil || !ok || strings.Contains(string(wire), "PRIVATE") || strings.Contains(string(wire), "Get-Content") || strings.Contains(string(wire), "tool_input") {
		t.Fatalf("unsafe wire %s", wire)
	}
	e, ok := events.Decode(wire, "")
	if !ok || e.Activity != status.Reading || e.ActivityConfirmed {
		t.Fatal(e)
	}
}
