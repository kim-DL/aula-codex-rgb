package status

import (
	"testing"
	"time"
)

func TestFileAfterglowSurvivesWrappersWithoutLosingLiveTools(t *testing.T) {
	for _, path := range []string{"notes.md", "main.go"} {
		e := New()
		at := time.Now()
		apply := func(ms int, kind, call string) {
			e.Apply(Event{Thread: "work", Turn: "one", Type: kind, Call: call, Tool: "exec", At: at.Add(time.Duration(ms) * time.Millisecond), ReceivedAt: at.Add(time.Duration(ms) * time.Millisecond)})
		}
		e.Apply(Event{Thread: "work", Turn: "one", Type: "FileChange", Path: path, At: at, ReceivedAt: at})
		apply(200, "PreToolUse", "a")
		apply(400, "PreToolUse", "b")
		apply(600, "PostToolUse", "a")
		apply(700, "AgentMessage", "")
		want := fileKind(path)
		task := e.Snapshot(at.Add(1700 * time.Millisecond))[0]
		if task.State != want || task.ActiveTools != 1 {
			t.Fatalf("cue/execution lost: %+v", task)
		}
		task = e.Snapshot(at.Add(1900 * time.Millisecond))[0]
		if task.State != Tool || task.ActiveTools != 1 {
			t.Fatalf("afterglow was extended by unrelated wrappers: %+v", task)
		}
	}
}

func TestUrgentAndTerminalSignalsPreemptAfterglow(t *testing.T) {
	for _, test := range []struct {
		kind string
		want Kind
	}{{"PermissionRequest", Approval}, {"task_complete", Complete}, {"Interrupt", Interrupted}} {
		e := New()
		at := time.Now()
		e.Apply(Event{Thread: "t", Turn: "1", Type: "FileChange", Path: "readme.md", At: at, ReceivedAt: at})
		e.Apply(Event{Thread: "t", Turn: "1", Type: test.kind, Call: "real", At: at.Add(100 * time.Millisecond), ReceivedAt: at.Add(100 * time.Millisecond)})
		if got := e.Snapshot(at.Add(200 * time.Millisecond))[0].State; got != test.want {
			t.Fatalf("%s was delayed by afterglow: %s", test.kind, got)
		}
	}
}

func TestResponseAfterglowAndToolPreemption(t *testing.T) {
	e := New()
	at := time.Now()
	e.Apply(Event{Thread: "t", Type: "AgentMessage", At: at, ReceivedAt: at})
	e.Apply(Event{Thread: "t", Type: "PostToolUse", At: at.Add(100 * time.Millisecond), ReceivedAt: at.Add(100 * time.Millisecond)})
	if e.Snapshot(at.Add(1500 * time.Millisecond))[0].State != Responding {
		t.Fatal("response cue cut by wrapper")
	}
	e.Apply(Event{Thread: "t", Type: "PreToolUse", Call: "real", At: at.Add(1550 * time.Millisecond), ReceivedAt: at.Add(1550 * time.Millisecond)})
	if e.Snapshot(at.Add(1550 * time.Millisecond))[0].State != Tool {
		t.Fatal("response cue hid a new execution")
	}
}
