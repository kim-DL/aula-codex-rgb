package status

import (
	"testing"
	"time"
)

func TestConcurrentExecutionsAndIndependentApprovals(t *testing.T) {
	e := New()
	n := time.Now()
	send := func(typ, call string, offset time.Duration) {
		e.Apply(Event{ID: typ + call, Thread: "t", Turn: "turn", Call: call, Type: typ, At: n.Add(offset), ReceivedAt: n.Add(offset)})
	}
	send("PreToolUse", "a", 0)
	send("PreToolUse", "b", time.Millisecond)
	send("PermissionRequest", "a", 2*time.Millisecond)
	send("PermissionRequest", "b", 3*time.Millisecond)
	send("PostToolUse", "a", 4*time.Millisecond)
	got := e.Snapshot(n.Add(time.Second))[0]
	if got.State != Approval || got.PendingApprovals != 1 || got.ActiveTools != 1 {
		t.Fatalf("unrelated wait cleared: %+v", got)
	}
	send("PreToolUse", "b", 2*time.Second)
	got = e.Snapshot(n.Add(2 * time.Second))[0]
	if got.State != Tool || got.PendingApprovals != 0 {
		t.Fatalf("approval not released on execution: %+v", got)
	}
	send("PostToolUse", "b", 3*time.Second)
	got = e.Snapshot(n.Add(4 * time.Second))[0]
	if got.State != Thinking || got.ActiveTools != 0 || got.Confirmed {
		t.Fatalf("not estimated thinking: %+v", got)
	}
}
func TestPostBeforePreAndTerminalTombstone(t *testing.T) {
	e := New()
	n := time.Now()
	e.Apply(Event{Thread: "t", Turn: "one", Call: "a", Type: "PostToolUse", At: n, ReceivedAt: n})
	if e.Apply(Event{Thread: "t", Turn: "one", Call: "a", Type: "PreToolUse", At: n, ReceivedAt: n.Add(time.Second)}) {
		t.Fatal("late start resurrected ended call")
	}
	e.Apply(Event{Thread: "t", Turn: "one", Type: "task_complete", At: n.Add(2 * time.Second), ReceivedAt: n.Add(2 * time.Second)})
	if len(e.Snapshot(n.Add(11*time.Second))) != 0 {
		t.Fatal("completion cue not released")
	}
	if e.Apply(Event{Thread: "t", Turn: "one", Call: "b", Type: "PreToolUse", At: n.Add(12 * time.Second)}) {
		t.Fatal("late tool resurrected removed task")
	}
	if !e.Apply(Event{Thread: "t", Turn: "two", Type: "UserPromptSubmit", At: n.Add(13 * time.Second)}) {
		t.Fatal("new turn blocked")
	}
	if e.Apply(Event{Thread: "t", Turn: "one", Type: "task_complete", At: n.Add(14 * time.Second)}) {
		t.Fatal("old turn completed new turn")
	}
}

func TestUnrelatedErrorCannotMaskApproval(t *testing.T) {
	e := New()
	n := time.Now()
	e.Apply(Event{Thread: "t", Call: "waiting", Type: "PermissionRequest", At: n, ReceivedAt: n})
	e.Apply(Event{Thread: "t", Call: "other", Type: "PostToolUse", Error: true, At: n.Add(time.Second), ReceivedAt: n.Add(time.Second)})
	if got := e.Snapshot(n.Add(time.Second))[0]; got.State != Approval || got.PendingApprovals != 1 {
		t.Fatal(got)
	}
}
func TestTransientFileCueAndFreshnessDoNotInventCompletion(t *testing.T) {
	e := New()
	n := time.Now()
	e.Apply(Event{Thread: "t", Type: "FileChange", Path: "notes.md", At: n, ReceivedAt: n})
	if e.Snapshot(n.Add(500 * time.Millisecond))[0].State != Editing {
		t.Fatal("file cue too short")
	}
	if got := e.Snapshot(n.Add(1900 * time.Millisecond))[0]; got.State != Thinking || got.Confirmed {
		t.Fatalf("file cue stuck: %+v", got)
	}
	e.Apply(Event{Thread: "wait", Type: "PermissionRequest", Call: "a", At: n, ReceivedAt: n})
	e.Apply(Event{Thread: "wait", Type: "PostToolUse", Call: "unrelated", At: n.Add(time.Second), ReceivedAt: n.Add(time.Second)})
	got := e.Snapshot(n.Add(10 * time.Minute))[0]
	if got.State != Approval || !got.Stale || !got.Confirmed {
		t.Fatalf("unanswered wait lost: %+v", got)
	}
}
func TestOutOfOrderIndependentCloseAndUnknownEvent(t *testing.T) {
	e := New()
	n := time.Now()
	e.Apply(Event{Thread: "t", Type: "UserPromptSubmit", At: n, ReceivedAt: n})
	e.Apply(Event{Thread: "t", Call: "a", Type: "PreToolUse", At: n.Add(time.Second), ReceivedAt: n.Add(time.Second)})
	e.Apply(Event{Thread: "t", Call: "b", Type: "PreToolUse", At: n.Add(3 * time.Second), ReceivedAt: n.Add(3 * time.Second)})
	if !e.Apply(Event{Thread: "t", Call: "a", Type: "PostToolUse", At: n.Add(2 * time.Second), ReceivedAt: n.Add(4 * time.Second)}) {
		t.Fatal("independent old close rejected")
	}
	if got := e.Snapshot(n.Add(5 * time.Second))[0]; got.State != Tool || got.ActiveTools != 1 {
		t.Fatalf("wrong active calls: %+v", got)
	}
	e.Apply(Event{Thread: "ghost", Type: "unknown", At: n})
	if len(e.Snapshot(n.Add(5*time.Second))) != 1 {
		t.Fatal("unknown event created ghost task")
	}
}
