//go:build windows

package aula

import (
	"encoding/json"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

type injectedRecoveryTransport struct {
	Transport
	kind              string
	injected          bool
	last              [reportSize]byte
	nativeApplyWrites int
	nativeErrors      int
}

func (t *injectedRecoveryTransport) SetFeatureReport(p [reportSize]byte) error {
	t.last = p
	if t.kind == "write" && !t.injected && p[0] == 4 && p[1] == 2 {
		t.injected = true
		return fmt.Errorf("controlled injected write failure: %w", syscall.Errno(31))
	}
	err := t.Transport.SetFeatureReport(p)
	if err != nil {
		t.nativeErrors++
	} else if p[0] == 4 && p[1] == 2 {
		t.nativeApplyWrites++
	}
	return err
}
func (t *injectedRecoveryTransport) GetFeatureReport() ([reportSize]byte, error) {
	if t.kind == "read" && !t.injected && t.last[0] == 4 && t.last[1] == 2 {
		t.injected = true
		return [reportSize]byte{}, fmt.Errorf("controlled injected read failure: %w", syscall.Errno(31))
	}
	p, err := t.Transport.GetFeatureReport()
	if err != nil {
		t.nativeErrors++
	}
	return p, err
}

// Explicit opt-in only. Normal builds/tests must never exercise USB hardware.
// The same Windows mutex excludes the tray app and other hardware trials.
func TestRealtimeHardwareRecoveryWithInjectedError(t *testing.T) {
	if os.Getenv("AULA_RGB_HARDWARE_RECOVERY") != "1" {
		t.Skip("requires explicit hardware opt-in")
	}
	name, err := syscall.UTF16PtrFromString(`Local\AulaCodexRgb-0C45-800A`)
	if err != nil {
		t.Fatal(err)
	}
	h, _, mutexErr := modKernel32.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		t.Fatal(mutexErr)
	}
	defer procCloseHandle.Call(h)
	if mutexErr == syscall.ERROR_ALREADY_EXISTS {
		t.Fatal("stop the tray app before the hardware recovery trial")
	}
	d, err := Open()
	if err != nil {
		t.Fatal(err)
	}
	idle := LEDFrame{}
	for _, idx := range realtimeLEDOrder {
		idle[idx] = [3]uint8{0, 60, 0}
	}
	record := map[string]any{"at": time.Now().UTC(), "kind": "controlled_error_injection_with_real_USB", "brightness_percent": 60, "scenarios": []map[string]any{}}
	defer func() {
		restoreErr := d.SendRealtimeFrame(idle)
		d.Close()
		record["green_restored"] = restoreErr == nil
		if restoreErr != nil {
			t.Error(restoreErr)
		}
		record["success"] = !t.Failed()
		if path := os.Getenv("AULA_RGB_RECOVERY_LOG"); path != "" {
			b, _ := json.MarshalIndent(record, "", "  ")
			if err := os.WriteFile(path, b, 0600); err != nil {
				t.Error(err)
			}
		}
	}()
	if err = d.SendRealtimeFrame(idle); err != nil {
		t.Fatal(err)
	}
	base := d.transport
	for _, kind := range []string{"write", "read"} {
		wrapped := &injectedRecoveryTransport{Transport: base, kind: kind}
		d.transport = wrapped
		frame := LEDFrame{}
		for idx, c := range idle {
			frame[idx] = c
		}
		frame[KeyF1] = [3]uint8{0, 0, 153}
		before := d.RealtimeStats()
		started := time.Now()
		err = d.SendRealtimeFrame(frame)
		elapsed := time.Since(started)
		d.transport = base
		if err != nil {
			t.Fatal(err)
		}
		after := d.RealtimeStats()
		if !wrapped.injected || wrapped.nativeApplyWrites != 1 {
			t.Fatal("successful apply write was duplicated or fault was not exercised")
		}
		if after.ApplyRecoveries-before.ApplyRecoveries != 1 {
			t.Fatal("injected fault was not recovered")
		}
		entry := map[string]any{"stage": kind, "injected_errors": 1, "native_errors": wrapped.nativeErrors, "native_successful_apply_writes": wrapped.nativeApplyWrites, "apply_attempt_errors": after.ApplyErrors - before.ApplyErrors, "transfer_ms": float64(elapsed.Microseconds()) / 1000, "success": true}
		record["scenarios"] = append(record["scenarios"].([]map[string]any), entry)
		t.Logf("%s injection recovered, real USB apply writes=%d, transfer=%s", kind, wrapped.nativeApplyWrites, elapsed)
		time.Sleep(time.Second)
		if err = d.SendRealtimeFrame(idle); err != nil {
			t.Fatal(err)
		}
	}
}
