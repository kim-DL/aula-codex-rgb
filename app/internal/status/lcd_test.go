package status

import (
	"testing"
	"time"
)

func TestLCDUsesAcceptedRealEventsAndIgnoresCompletionAfterglow(t *testing.T) {
	e := New()
	e.Clock().SetEnabled(true)
	now := time.Now()
	start := Event{ID: "start", Thread: "t", Turn: "1", Type: "UserPromptSubmit", At: now, ReceivedAt: now}
	temporary := start
	temporary.Temporary = true
	if e.Apply(temporary) || e.Clock().Status(now).Active {
		t.Fatal("temporary timer")
	}
	if !e.Apply(start) || !e.Clock().Status(now).Active {
		t.Fatal("no accepted start")
	}
	if e.Apply(start) {
		t.Fatal("duplicate accepted")
	}
	stop := start
	stop.ID = "stop"
	stop.Type = "Stop"
	stop.At = now.Add(time.Second)
	stop.ReceivedAt = stop.At
	e.Apply(stop)
	if !e.Clock().Status(stop.At).Active {
		t.Fatal("Stop ended LCD")
	}
	end := stop
	end.ID = "end"
	end.Type = "task_complete"
	end.At = now.Add(2 * time.Second)
	end.ReceivedAt = end.At
	if !e.Apply(end) {
		t.Fatal("completion rejected")
	}
	if s := e.Clock().Status(end.At); s.Active || s.DesiredMode != "pc_clock" || s.RecentSeconds != 2 {
		t.Fatal(s)
	}
	if tasks := e.Snapshot(end.At); len(tasks) != 1 || tasks[0].State != Complete {
		t.Fatal("RGB completion was changed", tasks)
	}
}

func TestLCDRecoveryRequiresMatchingLiveTurn(t *testing.T) {
	e := New()
	e.Clock().SetEnabled(true)
	now := time.Now()
	start := Event{Thread: "t", Turn: "1", Type: "UserPromptSubmit", Source: "rollout", At: now.Add(-10 * time.Second), ReceivedAt: now}
	if e.RecoverClockStart(start) {
		t.Fatal("replayed idle history")
	}
	tool := Event{Thread: "t", Turn: "1", Type: "PreToolUse", At: now, ReceivedAt: now}
	e.Apply(tool)
	wrong := start
	wrong.Turn = "old"
	if e.RecoverClockStart(wrong) {
		t.Fatal("unrelated turn")
	}
	temp := start
	temp.Temporary = true
	if e.RecoverClockStart(temp) {
		t.Fatal("temporary")
	}
	if !e.RecoverClockStart(start) || e.Clock().Status(now).ElapsedSeconds != 10 {
		t.Fatal("could not recover exact active turn")
	}
	end := tool
	end.Type = "task_complete"
	end.At = now.Add(time.Second)
	end.ReceivedAt = end.At
	e.Apply(end)
	if e.RecoverClockStart(start) || e.Clock().Status(now).Active {
		t.Fatal("resurrected complete turn")
	}
}
