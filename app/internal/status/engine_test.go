package status

import (
	"testing"
	"time"
)

func TestLifecycleDedupAndAuthoritativeCompletion(t *testing.T) {
	e := New()
	now := time.Now()
	apply := func(id, typ string) {
		e.Apply(Event{ID: id, Thread: "t", Turn: "1", Type: typ, At: now, Confirmed: true})
	}
	apply("u", "UserPromptSubmit")
	if got := e.Snapshot(now)[0].State; got != Thinking {
		t.Fatal(got)
	}
	apply("p", "PreToolUse")
	apply("p", "PreToolUse")
	if got := e.Snapshot(now)[0].State; got != Tool {
		t.Fatal(got)
	}
	apply("s", "Stop")
	if got := e.Snapshot(now)[0].State; got != Tool {
		t.Fatalf("Stop replaced active tool: %s", got)
	}
	apply("c", "task_complete")
	if got := e.Snapshot(now)[0].State; got != Complete {
		t.Fatal(got)
	}
	if len(e.seen) != 4 {
		t.Fatalf("duplicate processed: %d", len(e.seen))
	}
}
func TestApprovalAndExpiredCompletion(t *testing.T) {
	e := New()
	now := time.Now()
	e.Apply(Event{Thread: "t", Type: "PermissionRequest", At: now})
	if e.Snapshot(now)[0].State != Approval {
		t.Fatal("approval lost")
	}
	e.Apply(Event{Thread: "t", Type: "task_complete", At: now})
	if got := e.Snapshot(now.Add(9 * time.Second)); len(got) != 0 {
		t.Fatalf("completion not expired: %+v", got)
	}
}

func TestApprovalWaitDoesNotExpireWithoutDecision(t *testing.T) {
	e := New()
	now := time.Now()
	e.Apply(Event{Thread: "waiting", Turn: "1", Type: "PermissionRequest", Tool: "Bash", At: now})
	e.Apply(Event{Thread: "other", Turn: "2", Type: "PreToolUse", At: now})
	if tasks := e.Snapshot(now.Add(time.Hour)); len(tasks) == 0 || tasks[0].Thread != "waiting" || tasks[0].State != Approval || !tasks[0].Confirmed {
		t.Fatalf("unanswered approval disappeared: %+v", tasks)
	}
	e.Apply(Event{Thread: "waiting", Turn: "1", Type: "PostToolUse", Tool: "Bash", At: now.Add(time.Hour)})
	if tasks := e.Snapshot(now.Add(time.Hour)); tasks[0].State == Approval {
		t.Fatal("finished command did not release approval state")
	}
}

func TestOldEventCannotUndoCompletionAndSessionEndKeepsCue(t *testing.T) {
	e := New()
	now := time.Now()
	e.Apply(Event{Thread: "t", Type: "task_complete", At: now})
	if e.Apply(Event{Thread: "t", Type: "PreToolUse", At: now.Add(-time.Second)}) {
		t.Fatal("old event accepted")
	}
	e.Apply(Event{Thread: "t", Type: "SessionEnd", At: now.Add(time.Second)})
	if got := e.Snapshot(now.Add(2 * time.Second)); len(got) != 1 || got[0].State != Complete {
		t.Fatalf("completion cue lost: %+v", got)
	}
	if len(e.Snapshot(now.Add(9*time.Second))) != 0 {
		t.Fatal("cue did not expire")
	}
}

func TestFastDocumentChangeHoldsCueAndToolErrorRecovers(t *testing.T) {
	e := New()
	now := time.Now()
	e.Apply(Event{Thread: "t", Type: "FileChange", Path: "notes.md", At: now, ReceivedAt: now})
	e.Apply(Event{Thread: "t", Type: "PostToolUse", At: now.Add(50 * time.Millisecond), ReceivedAt: now.Add(50 * time.Millisecond)})
	if e.Snapshot(now.Add(400 * time.Millisecond))[0].State != Editing {
		t.Fatal("document cue disappeared before transition completed")
	}
	if e.Snapshot(now.Add(1900 * time.Millisecond))[0].State != Thinking {
		t.Fatal("cue did not return to estimated thinking")
	}
	e.Apply(Event{Thread: "t", Type: "PostToolUse", Error: true, At: now.Add(time.Second), ReceivedAt: now.Add(time.Second)})
	e.Apply(Event{Thread: "t", Type: "PostToolUse", At: now.Add(1100 * time.Millisecond), ReceivedAt: now.Add(1100 * time.Millisecond)})
	if e.Snapshot(now.Add(2 * time.Second))[0].State != Failed {
		t.Fatal("outer tool wrapper hid real failure")
	}
	if e.Snapshot(now.Add(6 * time.Second))[0].State != Thinking {
		t.Fatal("recoverable tool failure removed active task")
	}
	e.Apply(Event{Thread: "t", Type: "PreToolUse", At: now.Add(7 * time.Second), ReceivedAt: now.Add(7 * time.Second)})
	if e.Snapshot(now.Add(7 * time.Second))[0].State != Tool {
		t.Fatal("next tool did not recover")
	}
}

func TestPatchWithoutPathPreservesKnownFileClassification(t *testing.T) {
	for _, tc := range []struct {
		path string
		want Kind
	}{{"notes.md", Editing}, {"script.py", Coding}} {
		e := New()
		now := time.Now()
		e.Apply(Event{Thread: "t", Type: "FileChange", Path: tc.path, At: now, ReceivedAt: now})
		e.Apply(Event{Thread: "t", Type: "PostToolUse", Tool: "apply_patch", At: now.Add(100 * time.Millisecond), ReceivedAt: now.Add(100 * time.Millisecond)})
		if got := e.Snapshot(now.Add(200 * time.Millisecond))[0].State; got != tc.want {
			t.Fatalf("%s classified %s", tc.path, got)
		}
		e.Apply(Event{Thread: "t", Type: "FileChange", Path: tc.path, Error: true, At: now.Add(time.Second), ReceivedAt: now.Add(time.Second)})
		if got := e.Snapshot(now.Add(time.Second))[0].State; got != Failed {
			t.Fatalf("failed patch classified %s", got)
		}
	}
}
