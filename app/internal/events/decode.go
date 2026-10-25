package events

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"aula-codex-rgb-lab/internal/status"
)

func first(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
func nested(m map[string]any, k string) map[string]any {
	if v, ok := m[k].(map[string]any); ok {
		return v
	}
	if text, ok := m[k].(string); ok {
		var v map[string]any
		if json.Unmarshal([]byte(text), &v) == nil {
			return v
		}
	}
	return nil
}
func firstChangePath(v any) string {
	if changes, ok := v.(map[string]any); ok {
		paths := make([]string, 0, len(changes))
		for path := range changes {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		if len(paths) > 0 {
			return paths[0]
		}
	}
	changes, _ := v.([]any)
	for _, change := range changes {
		if m, ok := change.(map[string]any); ok {
			if path := first(m, "path", "file_path"); path != "" {
				return path
			}
		}
	}
	return ""
}
func firstPatchPath(patch string) string {
	for _, line := range strings.Split(patch, "\n") {
		for _, prefix := range []string{"*** Add File:", "*** Update File:", "*** Delete File:"} {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
	}
	return ""
}
func stamp(v any) time.Time {
	if s, ok := v.(string); ok {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t
	}
	return time.Time{}
}
func id(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:12])
}

// Decode accepts only event metadata. User message bodies, tool inputs and
// outputs are never copied into a normalized event or a log.
func Decode(raw []byte, path string) (status.Event, bool) {
	return decode(raw, path, "")
}

func DecodeForSession(raw []byte, path, sessionID string) (status.Event, bool) {
	return decode(raw, path, sessionID)
}

func decode(raw []byte, path, sessionID string) (status.Event, bool) {
	var root map[string]any
	if json.Unmarshal(raw, &root) != nil {
		return status.Event{}, false
	}
	kind := first(root, "type", "hook_event_name", "hookEventName")
	// Only an explicit hook field establishes a session without a transcript.
	// Missing fields in older relays and rollout records remain compatible.
	temporary, _ := root["temporary_session"].(bool)
	if first(root, "hook_event_name", "hookEventName") != "" {
		if transcript, present := root["transcript_path"]; present {
			if transcript == nil {
				temporary = true
			} else if path, ok := transcript.(string); ok && strings.TrimSpace(path) == "" {
				temporary = true
			}
		}
	}
	recorded := stamp(root["timestamp"])
	payload := nested(root, "payload")
	itemID := ""
	callID := ""
	responseTool := ""
	if kind == "event_msg" && len(payload) > 0 {
		kind = first(payload, "type")
		root = payload
	}
	if kind == "response_item" && len(payload) > 0 {
		responseTool = first(payload, "name")
		callID = first(payload, "call_id")
		switch first(payload, "type") {
		case "custom_tool_call", "function_call":
			kind = "PreToolUse"
			if strings.HasSuffix(responseTool, "request_user_input") || strings.HasSuffix(responseTool, "request_user_input_async") {
				kind = "PermissionRequest"
			}
		case "custom_tool_call_output", "function_call_output":
			kind = "PostToolUse"
		case "message":
			if first(payload, "role") == "assistant" {
				kind = "AgentMessage"
				itemID = first(payload, "id")
			}
		}
		root = payload
	}
	if kind == "task_started" || kind == "user_message" {
		kind = "UserPromptSubmit"
	}
	if kind == "turn_aborted" {
		kind = "Interrupt"
	}
	if kind == "error" || kind == "stream_error" {
		kind = "task_complete"
		root["error"] = true
	}
	thread := first(root, "thread_id", "threadId", "session_id", "sessionId")
	if thread == "" {
		thread = sessionID
	}
	if thread == "" {
		base := filepath.Base(path)
		if strings.HasPrefix(base, "rollout-") {
			base = strings.TrimSuffix(strings.TrimPrefix(base, "rollout-"), ".jsonl")
			if len(base) >= 36 {
				thread = base[len(base)-36:]
			}
		}
	}
	turn := first(root, "turn_id", "turnId")
	tool := first(root, "tool_name", "toolName")
	if tool == "" {
		tool = responseTool
	}
	toolInput := nested(root, "tool_input")
	if tool == "" {
		tool = first(root, "tool")
	}
	file := first(toolInput, "file_path", "path")
	if file == "" {
		file = first(root, "file_path", "path")
	}
	if file == "" && (tool == "apply_patch" || tool == "Write" || tool == "Edit" || strings.HasSuffix(tool, "apply_patch")) {
		patch := first(toolInput, "input", "patch", "patchText")
		if patch == "" {
			patch = first(root, "input", "patch")
		}
		if patch == "" {
			patch, _ = root["tool_input"].(string)
		}
		file = firstPatchPath(patch)
	}
	errFlag := false
	if n, ok := root["exit_code"].(float64); ok && n != 0 {
		errFlag = true
	}
	if s := first(root, "status"); s == "failed" || s == "error" {
		errFlag = true
	}
	if value, ok := root["error"]; ok && value != nil && value != false {
		errFlag = true
	}
	if response := nested(root, "tool_response"); len(response) > 0 {
		if s := first(response, "status"); s == "failed" || s == "error" {
			errFlag = true
		}
		if value, ok := response["error"]; ok && value != nil && value != false {
			errFlag = true
		}
		if n, ok := response["exit_code"].(float64); ok && n != 0 {
			errFlag = true
		}
		if response["isError"] == true || response["is_error"] == true {
			errFlag = true
		}
	}
	if kind == "item_completed" || kind == "item_started" {
		item := nested(root, "item")
		itemID = first(item, "id")
		switch first(item, "type") {
		case "AgentMessage", "agentMessage":
			kind = "AgentMessage"
		case "FileChange", "fileChange":
			kind = "FileChange"
			file = firstChangePath(item["changes"])
		case "CommandExecution", "commandExecution":
			kind = "PostToolUse"
			tool = "Bash"
			if first(item, "status") == "inProgress" {
				kind = "PreToolUse"
			}
		case "McpToolCall", "mcpToolCall":
			kind = "PostToolUse"
			tool = first(item, "tool")
			if first(item, "status") == "inProgress" {
				kind = "PreToolUse"
			}
		case "CollabToolCall", "collabToolCall":
			kind = "PostToolUse"
			tool = "Agent"
		case "DynamicToolCall", "dynamicToolCall":
			kind = "PostToolUse"
			tool = first(item, "tool")
		}
		if first(item, "status") == "failed" {
			errFlag = true
		}
		if n, ok := item["exit_code"].(float64); ok && n != 0 {
			errFlag = true
		}
	}
	if kind == "task_complete" && thread == "" {
		thread = first(root, "root_turn_id")
	}
	if thread == "" {
		return status.Event{}, false
	}
	subagent := first(root, "agent_id", "agentId", "subagent_id", "subagentId")
	if strings.HasPrefix(kind, "Subagent") && subagent != "" {
		thread += "#subagent:" + subagent
	}
	known := map[string]bool{"turn/started": true, "UserPromptSubmit": true, "SessionStart": true, "SessionEnd": true, "PreToolUse": true, "PostToolUse": true, "PermissionRequest": true, "Stop": true, "Interrupt": true, "SubagentStop": true, "SubagentStart": true, "AgentMessage": true, "FileChange": true, "task_complete": true, "turn/completed": true}
	if !known[kind] {
		return status.Event{}, false
	}
	source := "hook"
	if path != "" {
		source = "rollout"
	}
	when := stamp(root["timestamp"])
	if when.IsZero() {
		when = recorded
	}
	if when.IsZero() {
		when = stamp(root["created_at"])
	}
	if when.IsZero() {
		when = time.Now()
	}
	eventID := first(root, "event_id")
	if eventID == "" {
		call := first(root, "tool_use_id", "toolUseId")
		if call == "" {
			call = callID
		}
		if call != "" {
			eventID = id(call, kind)
		} else if itemID != "" {
			eventID = id(thread, turn, itemID, kind)
		} else {
			eventID = id(thread, turn, kind, tool, when.UTC().Truncate(250*time.Millisecond).Format(time.RFC3339Nano))
		}
	}
	call := first(root, "tool_use_id", "toolUseId", "call_id", "item_id")
	if call == "" {
		call = callID
	}
	if call == "" {
		call = itemID
	}
	// Scope phase deduplication to the conversation. Hooks and persisted records
	// can omit turn_id, but must agree on a tool's call identifier.
	if call != "" && (kind == "PreToolUse" || kind == "PostToolUse" || kind == "PermissionRequest" || kind == "FileChange") {
		eventID = id(thread, call, kind)
	}
	activity := status.Kind("")
	activityConfirmed := false
	if kind == "PreToolUse" || kind == "PostToolUse" {
		if first(root, "activity") == string(status.Reading) {
			activity = status.Reading
			activityConfirmed, _ = root["activity_confirmed"].(bool)
		} else if yes, exact := readingActivity(tool, root); yes {
			activity, activityConfirmed = status.Reading, exact
		}
	}
	return status.Event{Activity: activity, ActivityConfirmed: activityConfirmed, ID: eventID, Call: call, Thread: thread, Turn: turn, Type: kind, Tool: tool, Path: file, Error: errFlag, Temporary: temporary, Source: source, At: when, ReceivedAt: time.Now(), Confirmed: kind != "UserPromptSubmit" && kind != "SessionStart"}, true
}
