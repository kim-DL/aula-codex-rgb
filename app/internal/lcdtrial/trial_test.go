package lcdtrial

import (
	"testing"
	"time"
)

func TestCountHoldOverwriteAndRestore(t *testing.T) {
	now := time.Date(2026, 10, 25, 12, 0, 0, 0, time.Local)
	x := New(now)
	if c, s, _ := x.Next(now, false); !s || !c.Equal(now) {
		t.Fatal("initial clock")
	}
	now = now.Add(10 * time.Second)
	c, s, _ := x.Next(now, false)
	if !s || c.Hour() != 0 || c.Second() != 0 || x.Phase() != "count_first" {
		t.Fatal("start")
	}
	now = now.Add(12 * time.Second)
	c, s, _ = x.Next(now, false)
	if !s || c.Second() != 12 || x.Recent() != 12*time.Second {
		t.Fatal("complete")
	}
	if _, s, _ = x.Next(now.Add(100*time.Millisecond), false); s {
		t.Fatal("unbounded refresh")
	}
	if c, s, _ = x.Next(now.Add(250*time.Millisecond), false); !s || c.Second() != 12 {
		t.Fatal("hold")
	}
	now = now.Add(15 * time.Second)
	c, s, _ = x.Next(now, false)
	if !s || c.Second() != 0 || x.Phase() != "count_new" {
		t.Fatal("restart")
	}
	now = now.Add(8 * time.Second)
	c, s, _ = x.Next(now, false)
	if !s || c.Second() != 8 || x.Recent() != 8*time.Second {
		t.Fatal("latest did not replace previous")
	}
	now = now.Add(15 * time.Second)
	c, s, _ = x.Next(now, false)
	if !s || !c.Equal(now) || x.Phase() != "restore_pc_clock" {
		t.Fatal("restore")
	}
	_, _, done := x.Next(now.Add(6*time.Second), false)
	if !done {
		t.Fatal("finish")
	}
}

func TestReconnectPreservesElapsedAndTodayDate(t *testing.T) {
	now := time.Date(2026, 10, 25, 23, 59, 50, 0, time.Local)
	x := New(now)
	x.Next(now, false)
	x.Next(now.Add(10*time.Second), false)
	c, s, _ := x.Next(now.Add(13*time.Second), true)
	if !s || c.Day() != 26 || c.Hour() != 0 || c.Minute() != 0 || c.Second() != 3 {
		t.Fatal("reconnect/date")
	}
}
