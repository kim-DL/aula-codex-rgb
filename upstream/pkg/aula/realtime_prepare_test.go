//go:build windows

package aula

import (
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestRealtimePreparationIsIdempotentAndPreservesWireSequence(t *testing.T) {
	tx := &retryTransport{}
	d := Device{transport: tx}
	if err := d.PrepareRealtime(); err != nil {
		t.Fatal(err)
	}
	if len(tx.sets) != 4 || !d.realtimeInitialized {
		t.Fatal("preparation wrote frame data or did not finish")
	}
	stats := d.RealtimeStats()
	if stats.InitializationAttempts != 1 || stats.InitializationNanos < uint64(200*time.Millisecond) {
		t.Fatal("firmware settling was shortened", stats)
	}
	if err := d.PrepareRealtime(); err != nil || len(tx.sets) != 4 {
		t.Fatal("idempotent preparation repeated handshake")
	}
	frame := LEDFrame{KeyF1: {0, 100, 0}}
	if err := d.SendRealtimeFrame(frame); err != nil {
		t.Fatal(err)
	}
	if d.RealtimeStats().InitializationAttempts != 1 {
		t.Fatal("frame restarted preparation")
	}
	profile := d.RealtimeStats().LastFrame
	if profile.InitializationNanos != 0 || profile.RequestedSleepNanos != uint64(36*time.Millisecond) || profile.ActualSleepNanos < profile.RequestedSleepNanos || profile.TotalNanos < profile.ReadNanos+profile.WriteNanos+profile.ActualSleepNanos {
		t.Fatalf("frame timings lost the unchanged firmware pacing: %+v", profile)
	}
	lazyTx := &retryTransport{}
	lazy := Device{transport: lazyTx}
	if err := lazy.SendRealtimeFrame(frame); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(tx.sets, lazyTx.sets) {
		t.Fatal("explicit preparation changed USB packets or order")
	}
}

type prepareFailureTransport struct {
	*retryTransport
	fail bool
}

func (t *prepareFailureTransport) SetFeatureReport(p [reportSize]byte) error {
	if t.fail {
		return syscall.Errno(5)
	}
	return t.retryTransport.SetFeatureReport(p)
}

func TestRealtimeFailedPreparationNeverClaimsReady(t *testing.T) {
	tx := &prepareFailureTransport{retryTransport: &retryTransport{}, fail: true}
	d := Device{transport: tx}
	if err := d.PrepareRealtime(); err == nil || d.realtimeInitialized {
		t.Fatal("failed initialization marked ready")
	}
	tx.fail = false
	if err := d.PrepareRealtime(); err != nil {
		t.Fatal(err)
	}
	if d.RealtimeStats().InitializationAttempts != 2 {
		t.Fatal("preparation attempts not recorded")
	}
}
