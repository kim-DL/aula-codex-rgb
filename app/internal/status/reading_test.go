package status

import (
	"testing"
	"time"
)

func TestReadCallPairHasWhiteCueWithoutBecomingFileModification(t *testing.T) {
	e := New()
	at := time.Now()
	pre := Event{ID: "pre", Thread: "t", Turn: "1", Call: "r", Type: "PreToolUse", Tool: "Read", Path: "app.go", Activity: Reading, ActivityConfirmed: true, At: at, ReceivedAt: at}
	e.Apply(pre)
	if s := e.Snapshot(at)[0]; s.State != Reading || s.ActiveTools != 1 || !s.Confirmed {
		t.Fatal(s)
	}
	// Desktop outputs can omit both tool name and input. Recover only by call ID.
	post := Event{ID: "post", Thread: "t", Turn: "1", Call: "r", Type: "PostToolUse", At: at.Add(time.Second), ReceivedAt: at.Add(time.Second)}
	e.Apply(post)
	if s := e.Snapshot(post.At.Add(time.Second))[0]; s.State != Reading || s.ActiveTools != 0 {
		t.Fatal(s)
	}
	if s := e.Snapshot(post.At.Add(1801 * time.Millisecond))[0]; s.State != Thinking {
		t.Fatal(s)
	}
	e.Apply(Event{Thread: "t", Turn: "1", Type: "FileChange", Path: "app.go", At: at.Add(4 * time.Second), ReceivedAt: at.Add(4 * time.Second)})
	if s := e.Snapshot(at.Add(4 * time.Second))[0]; s.State != Coding {
		t.Fatal("write classification changed", s)
	}
}
func TestReadConfidenceConcurrencyApprovalAndFailure(t *testing.T) {
	e := New()
	at := time.Now()
	apply := func(id, call, kind string, read, failed bool, ms int) {
		e.Apply(Event{ID: id, Thread: "t", Turn: "1", Call: call, Type: kind, Tool: "exec", Activity: func() Kind {
			if read {
				return Reading
			}
			return ""
		}(), Error: failed, At: at.Add(time.Duration(ms) * time.Millisecond), ReceivedAt: at.Add(time.Duration(ms) * time.Millisecond)})
	}
	apply("r", "r", "PreToolUse", true, false, 0)
	apply("b", "b", "PreToolUse", false, false, 100)
	if s := e.Snapshot(at.Add(time.Second))[0]; s.State != Reading || s.ActiveTools != 2 || s.Confirmed {
		t.Fatal(s)
	}
	apply("a", "b", "PermissionRequest", false, false, 1100)
	if s := e.Snapshot(at.Add(1100 * time.Millisecond))[0]; s.State != Approval {
		t.Fatal(s)
	}
	apply("rd", "r", "PostToolUse", false, false, 1200)
	if s := e.Snapshot(at.Add(1200 * time.Millisecond))[0]; s.State != Approval {
		t.Fatal(s)
	}
	apply("bd", "b", "PostToolUse", false, true, 1300)
	if s := e.Snapshot(at.Add(1300 * time.Millisecond))[0]; s.State != Failed {
		t.Fatal(s)
	}
	apply("r2", "r2", "PreToolUse", true, false, 1400)
	if s := e.Snapshot(at.Add(1400 * time.Millisecond))[0]; s.State != Failed {
		t.Fatal("read masked failure", s)
	}
	apply("r2d", "r2", "PostToolUse", false, false, 1500)
	if s := e.Snapshot(at.Add(1500 * time.Millisecond))[0]; s.State != Failed {
		t.Fatal(s)
	}
}

func TestReadRolloutCanEnrichOnlyMatchingOutstandingCall(t *testing.T) {
	e := New()
	at := time.Now()
	v := Event{ID: "call-pre", Thread: "t", Turn: "1", Call: "c", Type: "PreToolUse", At: at, ReceivedAt: at}
	e.Apply(v)
	v.Activity = Reading
	v.Source = "rollout"
	wrong := v
	wrong.Turn = "old"
	if e.Apply(wrong) {
		t.Fatal("different turn enriched current call")
	}
	if !e.Apply(v) {
		t.Fatal("richer metadata rejected")
	}
	if s := e.Snapshot(at)[0]; s.State != Reading || s.ActiveTools != 1 || s.Confirmed {
		t.Fatal(s)
	}
	if e.Apply(v) {
		t.Fatal("duplicate read accepted twice")
	}
	e.Apply(Event{ID: "done", Thread: "t", Turn: "1", Call: "c", Type: "PostToolUse", At: at.Add(time.Second), ReceivedAt: at.Add(time.Second)})
	if e.Apply(v) {
		t.Fatal("completed call revived")
	}
}
