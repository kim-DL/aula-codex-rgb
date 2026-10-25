package events

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDecodeHookDropsPromptAndCommandText(t *testing.T) {
	input := map[string]any{"hook_event_name": "PreToolUse", "session_id": "thread-1", "turn_id": "turn-2", "tool_name": "Bash", "tool_use_id": "call-3", "tool_input": map[string]any{"command": "SECRET-command-text"}}
	raw, _ := json.Marshal(input)
	e, ok := Decode(raw, "")
	if !ok || e.Type != "PreToolUse" || e.Thread != "thread-1" || e.Call != "call-3" {
		t.Fatalf("decoded=%+v ok=%t", e, ok)
	}
	out, _ := json.Marshal(e)
	if strings.Contains(string(out), "SECRET-command-text") {
		t.Fatal("private command leaked into event")
	}
}

func TestDesktopCallStartAndEndPreserveTimestampAndDropContents(t *testing.T) {
	start := []byte(`{"timestamp":"2026-10-14T06:38:31.199Z","type":"response_item","payload":{"type":"custom_tool_call","call_id":"real-call","name":"exec","input":"PRIVATE-COMMAND"}}`)
	end := []byte(`{"timestamp":"2026-10-14T06:38:43.756Z","type":"response_item","payload":{"type":"custom_tool_call_output","call_id":"real-call","output":"PRIVATE-OUTPUT"}}`)
	a, ok := DecodeForSession(start, "desktop.jsonl", "desktop-session")
	if !ok || a.Type != "PreToolUse" || a.Tool != "exec" || a.Thread != "desktop-session" {
		t.Fatalf("start: %+v, %t", a, ok)
	}
	b, ok := DecodeForSession(end, "desktop.jsonl", "desktop-session")
	if !ok || b.Type != "PostToolUse" || a.ID == b.ID || a.Call != b.Call || b.Call != "real-call" {
		t.Fatalf("end: %+v, %t", b, ok)
	}
	want, _ := time.Parse(time.RFC3339Nano, "2026-10-14T06:38:31.199Z")
	if !a.At.Equal(want) {
		t.Fatalf("outer timestamp lost: %s", a.At)
	}
	encoded, _ := json.Marshal([]any{a, b})
	if strings.Contains(string(encoded), "PRIVATE-") {
		t.Fatal("tool contents retained")
	}
}

func TestDesktopFileChangeMap(t *testing.T) {
	raw := []byte(`{"timestamp":"2026-10-14T06:38:31.894Z","type":"event_msg","payload":{"type":"item_completed","thread_id":"desktop-session","turn_id":"t","item":{"type":"FileChange","id":"patch-1","status":"completed","changes":{"C:\\Lab\\sample.py":{"type":"add","content":"PRIVATE-FILE"}}}}}`)
	e, ok := DecodeForSession(raw, "desktop.jsonl", "desktop-session")
	if !ok || e.Type != "FileChange" || e.Path != `C:\Lab\sample.py` {
		t.Fatalf("file event: %+v %t", e, ok)
	}
	b, _ := json.Marshal(e)
	if strings.Contains(string(b), "PRIVATE-FILE") {
		t.Fatal("file contents retained")
	}
}

func TestPatchHookExtractsOnlyTargetPath(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PostToolUse","session_id":"s","tool_name":"apply_patch","tool_input":{"input":"*** Begin Patch\n*** Add File: notes.md\n+PRIVATE-CONTENT\n*** End Patch"},"tool_response":{"exit_code":0,"error":null},"error":false}`)
	e, ok := Decode(raw, "")
	if !ok || e.Path != "notes.md" || e.Error {
		t.Fatalf("patch metadata: %+v %t", e, ok)
	}
	b, _ := json.Marshal(e)
	if strings.Contains(string(b), "PRIVATE-CONTENT") {
		t.Fatal("patch content retained")
	}
}

func TestFreeformHookStringInputAndJSONResponse(t *testing.T) {
	raw := []byte(`{"hook_event_name":"PostToolUse","session_id":"s","tool_name":"apply_patch","tool_input":"*** Begin Patch\n*** Update File: notes.md\n@@\n+PRIVATE-TEXT\n*** End Patch","tool_response":"{\"exit_code\":7}"}`)
	e, ok := Decode(raw, "")
	if !ok || e.Path != "notes.md" || !e.Error {
		t.Fatalf("freeform metadata: %+v %t", e, ok)
	}
	b, _ := json.Marshal(e)
	if strings.Contains(string(b), "PRIVATE-TEXT") {
		t.Fatal("freeform contents retained")
	}
}

func TestDecodeRolloutCompletionUsesFilenameThread(t *testing.T) {
	input := map[string]any{"type": "event_msg", "payload": map[string]any{"type": "task_complete", "root_turn_id": "turn-9"}}
	raw, _ := json.Marshal(input)
	e, ok := Decode(raw, `C:\sessions\rollout-2026-10-14T13-50-03-00000000-0000-4000-8000-000000000001.jsonl`)
	if !ok || e.Type != "task_complete" || e.Thread != "00000000-0000-4000-8000-000000000001" {
		t.Fatalf("decoded=%+v ok=%t", e, ok)
	}
}

func TestDecodeDistinctToolPhasesAndAssistantMessageWithoutContent(t *testing.T) {
	pre, ok := Decode([]byte(`{"hook_event_name":"PreToolUse","session_id":"thread-1","turn_id":"turn-1","tool_name":"Bash","tool_use_id":"call-7","tool_input":{"command":"private"}}`), "")
	if !ok {
		t.Fatal("pre-tool event was not decoded")
	}
	post, ok := Decode([]byte(`{"hook_event_name":"PostToolUse","session_id":"thread-1","turn_id":"turn-1","tool_name":"Bash","tool_use_id":"call-7","tool_response":{"exit_code":0}}`), "")
	if !ok || pre.ID == post.ID {
		t.Fatalf("tool phases not distinguished: pre=%+v post=%+v", pre, post)
	}
	line := []byte(`{"type":"response_item","timestamp":"2026-10-14T12:00:00Z","payload":{"type":"message","id":"m-1","role":"assistant","phase":"final_answer","content":[{"type":"output_text","text":"private answer"}]}}`)
	message, ok := Decode(line, `C:\sessions\rollout-2026-10-14T12-00-00-00000000-0000-0000-0000-000000000001.jsonl`)
	if !ok || message.Type != "AgentMessage" {
		t.Fatalf("assistant message not classified: %+v ok=%t", message, ok)
	}
	out, _ := json.Marshal(message)
	if strings.Contains(string(out), "private answer") {
		t.Fatal("assistant body leaked into normalized event")
	}
}
