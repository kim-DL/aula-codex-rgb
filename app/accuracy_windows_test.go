//go:build windows && !rgb_gui

package main

import (
	"aula-codex-rgb-lab/internal/events"
	"aula-codex-rgb-lab/internal/status"
	"strings"
	"testing"
	"time"
)

func TestHookRelayRetainsCallAndDeduplicatesRollout(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PreToolUse","session_id":"s","tool_use_id":"a","tool_name":"exec","tool_input":{"command":"PRIVATE"},"timestamp":"2026-10-15T05:00:00Z"}`)
	wire, ok, err := hookMetadata(raw)
	if err != nil || !ok || strings.Contains(string(wire), "PRIVATE") {
		t.Fatalf("wire %s %v", wire, err)
	}
	hook, ok := events.Decode(wire, "")
	if !ok || hook.Call != "a" {
		t.Fatal(hook)
	}
	record, ok := events.DecodeForSession([]byte(`{"type":"response_item","timestamp":"2026-10-15T05:00:00Z","payload":{"type":"function_call","name":"exec","call_id":"a","arguments":"PRIVATE"}}`), "fixture.jsonl", "s")
	if !ok || hook.ID != record.ID {
		t.Fatalf("cross-source IDs differ: %+v %+v", hook, record)
	}
	e := status.New()
	e.Apply(hook)
	if e.Apply(record) {
		t.Fatal("duplicate execution accepted")
	}
	if got := e.Snapshot(time.Date(2026, 10, 15, 5, 0, 1, 0, time.UTC))[0]; got.ActiveTools != 1 {
		t.Fatal(got)
	}
}

func TestRelayPreservesTemporaryScopeWithoutPrivatePayload(t *testing.T) {
	for _, transcript := range []string{"null", `"C:\\private\\rollout.jsonl"`} {
		raw := []byte(`{"session_id":"s","turn_id":"t","hook_event_name":"UserPromptSubmit","transcript_path":` + transcript + `,"prompt":"PRIVATE-PROMPT"}`)
		wire, ok, err := hookMetadata(raw)
		if err != nil || !ok || strings.Contains(string(wire), "PRIVATE") || strings.Contains(string(wire), "private") || strings.Contains(string(wire), "transcript_path") {
			t.Fatalf("unsafe metadata %s error=%v", wire, err)
		}
		v, ok := events.Decode(wire, "")
		if !ok || v.Temporary != (transcript == "null") {
			t.Fatalf("relay lost scope: %s", wire)
		}
	}
}
