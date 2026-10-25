package lcdclock

import (
	"errors"
	"testing"
	"time"
)

func fixture() (*Clock, time.Time) {
	c := New()
	c.SetEnabled(true)
	return c, time.Date(2026, 10, 25, 13, 24, 5, 0, time.FixedZone("KST", 9*3600))
}
func event(thread, turn, typ string, at time.Time) Event {
	return Event{thread, turn, typ, "test", at, at}
}
func commit(t *testing.T, c *Clock, at time.Time, mode string) Plan {
	t.Helper()
	p, ok := c.Next(at, false)
	if !ok || p.Mode != mode {
		t.Fatalf("plan %v %v, want %s", p, ok, mode)
	}
	c.Commit(p, at, nil)
	return p
}
func TestBoundaryOnlyWritesAndSingleRecentResult(t *testing.T) {
	c, now := fixture()
	commit(t, c, now, "pc_clock")
	c.Accept(event("a", "1", "UserPromptSubmit", now))
	p := commit(t, c, now.Add(3*time.Second), "elapsed")
	if p.Time.Hour() != 0 || p.Time.Minute() != 0 || p.Time.Second() != 3 || p.Time.Day() != 25 {
		t.Fatal(p)
	}
	for _, typ := range []string{"Stop", "PostToolUse", "AgentMessage"} {
		c.Accept(event("a", "1", typ, now.Add(4*time.Second)))
	}
	if _, ok := c.Next(now.Add(5*time.Second), false); ok {
		t.Fatal("per-second or Stop rewrite")
	}
	c.Accept(event("a", "1", "task_complete", now.Add(12*time.Second)))
	p = commit(t, c, now.Add(13*time.Second), "pc_clock")
	if p.Time.Hour() != 13 || p.Time.Minute() != 24 || p.Time.Second() != 18 {
		t.Fatal(p)
	}
	s := c.Status(now.Add(13 * time.Second))
	if s.Active || !s.HasRecent || s.RecentSeconds != 12 {
		t.Fatal(s)
	}
	c.Accept(event("a", "2", "UserPromptSubmit", now.Add(14*time.Second)))
	c.Accept(event("a", "2", "Interrupt", now.Add(22*time.Second)))
	s = c.Status(now.Add(22 * time.Second))
	if s.Active || s.RecentSeconds != 8 {
		t.Fatal(s)
	}
}
func TestLatestRequestDuplicateOldEndAndCommitRace(t *testing.T) {
	c, now := fixture()
	c.Accept(event("a", "1", "UserPromptSubmit", now))
	c.Accept(event("a", "1", "UserPromptSubmit", now.Add(time.Second)))
	if c.Status(now.Add(2*time.Second)).ElapsedSeconds != 2 {
		t.Fatal("duplicate reset")
	}
	old, _ := c.Next(now, false)
	c.Accept(event("b", "2", "UserPromptSubmit", now.Add(3*time.Second)))
	c.Commit(old, now.Add(3*time.Second), nil)
	c.Accept(event("a", "1", "task_complete", now.Add(4*time.Second)))
	s := c.Status(now.Add(5 * time.Second))
	if !s.Active || s.Thread != "b" || s.ElapsedSeconds != 2 {
		t.Fatal(s)
	}
	commit(t, c, now.Add(5*time.Second), "elapsed")
	c.Accept(event("b", "old", "task_complete", now.Add(6*time.Second)))
	if !c.Status(now).Active {
		t.Fatal("different turn stopped current")
	}
	c.Accept(event("b", "2", "turn/completed", now.Add(7*time.Second)))
	commit(t, c, now.Add(7*time.Second), "pc_clock")
	c.Accept(event("a", "old", "UserPromptSubmit", now.Add(time.Second)))
	if c.Status(now).Active {
		t.Fatal("old start replay")
	}
}
func TestExclusionsAndDesktopGate(t *testing.T) {
	c := New()
	now := time.Now()
	c.Accept(event("a", "1", "UserPromptSubmit", now))
	if c.Status(now).Active {
		t.Fatal("CLI-only start")
	}
	c.SetEnabled(true)
	for _, e := range []Event{event("a", "1", "SessionStart", now), event("a#subagent:one", "1", "UserPromptSubmit", now), event("a", "1", "SubagentStart", now)} {
		c.Accept(e)
	}
	if c.Status(now).Active {
		t.Fatal("internal session start")
	}
	c.Accept(event("a", "1", "UserPromptSubmit", now))
	c.SetEnabled(false)
	if s := c.Status(now); s.Active || s.DesiredMode != "pc_clock" || s.HasRecent {
		t.Fatal(s)
	}
}
func TestReconnectMidnightAnd24HourLimit(t *testing.T) {
	c, now := fixture()
	now = time.Date(2026, 10, 25, 23, 59, 58, 0, now.Location())
	c.Accept(event("a", "1", "UserPromptSubmit", now))
	commit(t, c, now, "elapsed")
	p, ok := c.Next(now.Add(5*time.Second), false)
	if !ok || p.Time.Day() != 26 || p.Time.Hour() != 0 || p.Time.Second() != 5 {
		t.Fatal(p, ok)
	}
	c.Commit(p, now.Add(5*time.Second), nil)
	p, ok = c.Next(now.Add(9*time.Second), true)
	if !ok || p.Time.Second() != 9 {
		t.Fatal("reconnect lost elapsed", p)
	}
	commit(t, c, now.Add(24*time.Hour), "pc_clock")
}
func TestBoundedClockErrorsDoNotFlood(t *testing.T) {
	c, now := fixture()
	for i := 1; i <= 3; i++ {
		p, ok := c.Next(now, true)
		if !ok {
			t.Fatal("missing retry", i)
		}
		c.Commit(p, now, errors.New("USB"))
		if _, ok := c.Next(now.Add(time.Second), true); ok {
			t.Fatal("reconnect bypassed backoff")
		}
		now = now.Add(time.Duration(i) * 5 * time.Second)
	}
	if _, ok := c.Next(now.Add(time.Hour), true); ok {
		t.Fatal("circuit breaker failed")
	}
	if s := c.Status(now); s.Errors != 3 || !s.Suspended {
		t.Fatal(s)
	}
}

func TestUnknownTurnCanLearnIdentityAndNextTurn(t *testing.T) {
	c, now := fixture()
	c.Accept(event("a", "", "UserPromptSubmit", now))
	c.Accept(event("a", "1", "turn/started", now.Add(time.Second)))
	if s := c.Status(now.Add(2 * time.Second)); s.Turn != "1" || s.ElapsedSeconds != 2 {
		t.Fatal(s)
	}
	c.Accept(event("a", "1", "task_complete", now.Add(3*time.Second)))
	c.Accept(event("a", "2", "UserPromptSubmit", now.Add(4*time.Second)))
	if s := c.Status(now.Add(5 * time.Second)); !s.Active || s.Turn != "2" || s.ElapsedSeconds != 1 {
		t.Fatal(s)
	}
}
