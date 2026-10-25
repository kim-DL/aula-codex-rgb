package status

import (
	"testing"
	"time"
)

func TestDelayedCompletionAfterStopHint(t *testing.T) {
	e := New()
	now := time.Now()
	e.Apply(Event{Thread: "work", Turn: "turn", Type: "UserPromptSubmit", At: now, ReceivedAt: now})
	// A hook with no original timestamp uses its delivery time, while the
	// authoritative persisted record carries its original event timestamp.
	e.Apply(Event{Thread: "work", Turn: "turn", Type: "Stop", At: now.Add(3 * time.Second), ReceivedAt: now.Add(3 * time.Second)})
	if !e.Apply(Event{Thread: "work", Turn: "turn", Type: "task_complete", At: now.Add(2 * time.Second), ReceivedAt: now.Add(4 * time.Second)}) {
		t.Fatal("authoritative completion discarded after a stop hint")
	}
	if tasks := e.Snapshot(now.Add(5 * time.Second)); len(tasks) != 1 || tasks[0].State != Complete {
		t.Fatalf("completion not shown: %+v", tasks)
	}
	if tasks := e.Snapshot(now.Add(13 * time.Second)); len(tasks) != 0 {
		t.Fatalf("finished turn still looks busy: %+v", tasks)
	}
}

func TestSameTurnCompletionClosesLaterDeliveredToolAndWait(t *testing.T) {
	for _, typ := range []string{"PreToolUse", "PostToolUse", "PermissionRequest", "AgentMessage"} {
		t.Run(typ, func(t *testing.T) {
			e := New()
			now := time.Now()
			e.Apply(Event{Thread: "codex", Turn: "one", Type: "UserPromptSubmit", At: now, ReceivedAt: now})
			e.Apply(Event{Thread: "codex", Turn: "one", Call: "call", Type: typ, At: now.Add(4 * time.Second), ReceivedAt: now.Add(4 * time.Second)})
			if !e.Apply(Event{Thread: "codex", Turn: "one", Type: "task_complete", At: now.Add(3 * time.Second), ReceivedAt: now.Add(5 * time.Second)}) {
				t.Fatal("same-turn completion discarded")
			}
			task := e.Snapshot(now.Add(6 * time.Second))[0]
			if task.State != Complete || task.ActiveTools != 0 || task.PendingApprovals != 0 {
				t.Fatalf("finished lifecycle retained work: %+v", task)
			}
			if e.Apply(Event{Thread: "codex", Turn: "one", Call: "late", Type: "PreToolUse", At: now.Add(7 * time.Second)}) {
				t.Fatal("late tool revived finished turn")
			}
			if !e.Apply(Event{Thread: "codex", Turn: "two", Type: "UserPromptSubmit", At: now.Add(3500 * time.Millisecond), ReceivedAt: now.Add(8 * time.Second)}) {
				t.Fatal("late old hook timestamp blocked a genuine next turn")
			}
		})
	}
}

func TestUnidentifiedOldCompletionCannotCloseNewerActivity(t *testing.T) {
	e := New()
	now := time.Now()
	e.Apply(Event{Thread: "t", Turn: "new", Type: "UserPromptSubmit", At: now, ReceivedAt: now})
	e.Apply(Event{Thread: "t", Turn: "new", Type: "PreToolUse", Call: "live", At: now.Add(4 * time.Second), ReceivedAt: now.Add(4 * time.Second)})
	if e.Apply(Event{Thread: "t", Type: "task_complete", At: now.Add(3 * time.Second), ReceivedAt: now.Add(5 * time.Second)}) {
		t.Fatal("unidentified completion closed potentially newer work")
	}
	if e.Apply(Event{Thread: "t", Turn: "old", Type: "task_complete", At: now.Add(6 * time.Second)}) {
		t.Fatal("another turn's completion closed active work")
	}
	if tasks := e.Snapshot(now.Add(7 * time.Second)); len(tasks) != 1 || tasks[0].ActiveTools != 1 {
		t.Fatalf("live tool lost: %+v", tasks)
	}
}
