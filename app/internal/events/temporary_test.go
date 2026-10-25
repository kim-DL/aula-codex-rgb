package events

import (
	"testing"
	"time"

	"aula-codex-rgb-lab/internal/status"
)

func TestTemporaryHookCannotKeepCompletedWorkBusy(t *testing.T) {
	e := status.New()
	now := time.Now()
	e.Apply(status.Event{Thread: "work", Turn: "real", Type: "UserPromptSubmit", At: now})
	e.Apply(status.Event{Thread: "work", Turn: "real", Type: "task_complete", At: now.Add(time.Second)})
	for _, kind := range []string{"UserPromptSubmit", "PreToolUse", "PermissionRequest", "Stop", "SessionEnd"} {
		raw := []byte(`{"session_id":"prediction","turn_id":"internal","hook_event_name":"` + kind + `","transcript_path":null}`)
		v, ok := Decode(raw, "")
		if !ok || !v.Temporary || e.Apply(v) {
			t.Fatalf("temporary %s was not excluded: %+v", kind, v)
		}
	}
	if tasks := e.Snapshot(now.Add(12 * time.Second)); len(tasks) != 0 {
		t.Fatalf("temporary work prevents idle: %+v", tasks)
	}
	if e.IgnoredTemporaryEvents() != 5 {
		t.Fatal("temporary exclusion is not observable")
	}
}

func TestTranscriptScopeAndBackwardCompatibility(t *testing.T) {
	for _, test := range []struct {
		name, fields string
		temporary    bool
	}{
		{"null", `,"transcript_path":null`, true},
		{"empty", `,"transcript_path":""`, true},
		{"blank", `,"transcript_path":"  "`, true},
		{"recorded", `,"transcript_path":"C:\\private\\rollout.jsonl"`, false},
		{"older-relay", "", false},
		{"normalized-temporary", `,"temporary_session":true`, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			v, ok := Decode([]byte(`{"session_id":"s","hook_event_name":"UserPromptSubmit"`+test.fields+`}`), "")
			if !ok || v.Temporary != test.temporary {
				t.Fatalf("scope %+v ok=%v", v, ok)
			}
		})
	}
	v, ok := DecodeForSession([]byte(`{"type":"event_msg","payload":{"type":"task_complete","turn_id":"real"}}`), "rollout.jsonl", "work")
	if !ok || v.Temporary {
		t.Fatal("authoritative completion without hook fields was filtered")
	}
}

func TestRealApprovalAndConcurrentWorkRemainVisible(t *testing.T) {
	e := status.New()
	now := time.Now()
	for _, thread := range []string{"work", "codex"} {
		v, ok := Decode([]byte(`{"session_id":"`+thread+`","turn_id":"t","hook_event_name":"PermissionRequest","tool_use_id":"call","transcript_path":"C:\\sessions\\record.jsonl"}`), "")
		if !ok || !e.Apply(v) {
			t.Fatal("real approval filtered")
		}
	}
	tasks := e.Snapshot(now.Add(10 * time.Minute))
	if len(tasks) != 2 || tasks[0].State != status.Approval || tasks[1].State != status.Approval {
		t.Fatalf("real waits were cleared: %+v", tasks)
	}
}

func TestTemporarySignalCannotCloseOrModifyRealSession(t *testing.T) {
	e := status.New()
	now := time.Now()
	e.Apply(status.Event{Thread: "work", Turn: "real", Call: "approval", Type: "PermissionRequest", At: now})
	for _, kind := range []string{"UserPromptSubmit", "SessionEnd", "task_complete"} {
		if e.Apply(status.Event{Thread: "work", Turn: "real", Temporary: true, Type: kind, At: now.Add(time.Second)}) {
			t.Fatal("temporary signal altered a real session")
		}
	}
	tasks := e.Snapshot(now.Add(20 * time.Minute))
	if len(tasks) != 1 || tasks[0].State != status.Approval || tasks[0].PendingApprovals != 1 {
		t.Fatalf("real approval was lost: %+v", tasks)
	}
}
