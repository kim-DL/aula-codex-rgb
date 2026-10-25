//go:build windows

package aula

import (
	"errors"
	"fmt"
	"syscall"
	"testing"
)

type retryTransport struct {
	sets             [][reportSize]byte
	last             [reportSize]byte
	failSet, failGet int
	code             syscall.Errno
}

func (t *retryTransport) SetFeatureReport(p [reportSize]byte) error {
	t.sets = append(t.sets, p)
	t.last = p
	if p[0] == 4 && p[1] == 2 && t.failSet > 0 {
		t.failSet--
		return fmt.Errorf("SetFeature: %w", t.code)
	}
	return nil
}
func (t *retryTransport) GetFeatureReport() ([reportSize]byte, error) {
	if t.last[0] == 4 && t.last[1] == 2 && t.failGet > 0 {
		t.failGet--
		return t.last, fmt.Errorf("GetFeature: %w", t.code)
	}
	return t.last, nil
}
func (*retryTransport) InitLCD() error              { panic("LCD accessed") }
func (*retryTransport) WriteLCDPage([]byte) error   { panic("LCD accessed") }
func (*retryTransport) ReadLCDAck() ([]byte, error) { panic("LCD accessed") }
func (*retryTransport) Close()                      {}

func TestRealtimeRecoveryIsBoundedAndOnlyReappliesFrame(t *testing.T) {
	for _, tc := range []struct {
		name                string
		set, get            int
		code                syscall.Errno
		attempts, recovered uint64
		fails               bool
	}{
		{"write transient", 1, 0, 31, 1, 1, false}, {"read transient", 0, 1, 31, 1, 1, false},
		{"persistent", 4, 0, 31, 3, 0, true}, {"access denied", 1, 0, 5, 1, 0, true},
		{"disconnected", 1, 0, 1167, 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			transport := &retryTransport{failSet: tc.set, failGet: tc.get, code: tc.code}
			d := Device{transport: transport, realtimeInitialized: true}
			err := d.SendRealtimeFrame(LEDFrame{KeyF1: {0, 153, 0}})
			if (err != nil) != tc.fails {
				t.Fatalf("error=%v", err)
			}
			if err != nil && !errors.Is(err, tc.code) {
				t.Fatal("error code lost")
			}
			stats := d.RealtimeStats()
			if stats.ApplyErrors != tc.attempts || stats.ApplyRecoveries != tc.recovered {
				t.Fatalf("stats=%+v", stats)
			}
			if tc.get > 0 {
				writes := 0
				for _, p := range transport.sets {
					if p[0] == 4 && p[1] == 2 {
						writes++
					}
				}
				if writes != 1 {
					t.Fatal("readback retry repeated a successful apply write")
				}
			}
			for i, p := range transport.sets {
				if i >= 10 && (p[0] != 4 || p[1] != 2) {
					t.Fatalf("retry changed command: %x", p[:9])
				}
				if p[0] == 4 && p[1] == 0xf0 {
					t.Fatal("flash command emitted")
				}
			}
			if len(transport.sets) > 12 {
				t.Fatal("unbounded retries")
			}
		})
	}
}
