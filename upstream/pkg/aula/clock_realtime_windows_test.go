//go:build windows

package aula

import (
	"testing"
	"time"
)

func TestRealtimeClockChangesOnlyCalendarBytes(t *testing.T) {
	stamp := time.Date(2026, 10, 25, 0, 0, 12, 0, time.FixedZone("KST", 9*3600))
	p, err := realtimeClockPackets(stamp)
	if err != nil {
		t.Fatal(err)
	}
	base := buildRealtimeHandshakePackets()
	for i := range p {
		for b := range p[i] {
			calendar := i == 2 && (b >= 3 && b <= 8 || b == 10)
			if !calendar && p[i][b] != base[i][b] {
				t.Fatalf("unexpected byte %d:%d", i, b)
			}
		}
	}
	want := [8]byte{26, 10, 25, 0, 0, 12, 0, 0}
	for i, v := range want {
		if p[2][3+i] != v {
			t.Fatalf("calendar byte %d", i)
		}
	}
	tx := &retryTransport{}
	d := Device{transport: tx, realtimeInitialized: true}
	if err := d.SetRealtimeClock(stamp); err != nil {
		t.Fatal(err)
	}
	if len(tx.sets) != 4 {
		t.Fatal("unexpected packet count")
	}
	for i := range p {
		if tx.sets[i] != p[i] {
			t.Fatal("wire sequence differs")
		}
	}
}

func TestRealtimeClockRejectsBeforeAnyDeviceWrite(t *testing.T) {
	tx := &retryTransport{}
	d := Device{transport: tx}
	if d.SetRealtimeClock(time.Now()) == nil || len(tx.sets) != 0 {
		t.Fatal("unprepared clock write")
	}
	d.realtimeInitialized = true
	if d.SetRealtimeClock(time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)) == nil || len(tx.sets) != 0 {
		t.Fatal("invalid year write")
	}
}
