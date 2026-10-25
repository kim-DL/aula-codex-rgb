package main

import (
	"testing"
	"time"
)

func TestReconnectRemainsAvailableAfterLongDisconnect(t *testing.T) {
	for failures := 1; failures <= 1000; failures++ {
		delay := reconnectDelay(failures)
		if delay < 3*time.Second || delay > 30*time.Second {
			t.Fatalf("unbounded reconnect delay at %d failures: %s", failures, delay)
		}
		if failures >= 10 && delay != 30*time.Second {
			t.Fatal("repeated failures did not enter cooldown")
		}
	}
	if reconnectDelay(1) != 3*time.Second {
		t.Fatal("retry burst did not reset after recovery")
	}
}

func TestScheduleGapUsesWallClockAcrossSuspend(t *testing.T) {
	now := time.Now()
	if scheduleGap(time.Time{}, now) || scheduleGap(now, now.Add(time.Second)) || scheduleGap(now, now.Add(-time.Second)) {
		t.Fatal("ordinary scheduling triggered reconnect")
	}
	if !scheduleGap(now, now.Add(30*time.Second)) {
		t.Fatal("suspend-sized gap did not invalidate handle")
	}
}
