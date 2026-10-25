//go:build windows

// A developer-only, RAM-only hardware trial. Standard output is an ephemeral
// status stream. Do not redirect it to a file. The installed RGB owner must be
// stopped first; its normal single-instance mutex and hooks are reused.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"aula-codex-rgb-lab/internal/bridge"
	"aula-codex-rgb-lab/internal/events"
	"aula-codex-rgb-lab/internal/instance"
	"aula-codex-rgb-lab/internal/lcdtrial"
	"aula-codex-rgb-lab/internal/status"
	"aula-codex-rgb-lab/internal/visual"
	"github.com/parsiya/f108-pro/pkg/aula"
)

func emit(v any) { _ = json.NewEncoder(os.Stdout).Encode(v) }

func run() (err error) {
	if events.DiagnosticsEnabled() {
		return fmt.Errorf("diagnostic persistence must be disabled")
	}
	release, ok, err := instance.Acquire()
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("another RGB owner is running")
	}
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	engine := status.New()
	pipeErrors := make(chan error, 8)
	go func() {
		if e := bridge.Serve(ctx, engine, pipeErrors); e != nil {
			select {
			case pipeErrors <- e:
			default:
			}
		}
	}()
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	started := time.Now()
	go func() {
		tail := events.NewTailer(filepath.Join(home, ".codex", "sessions"))
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for _, task := range engine.Snapshot(time.Now()) {
					tail.WatchThread(task.Thread)
				}
				_ = tail.Poll(func(b []byte, p, id string) {
					if ev, ok := events.DecodeForSession(b, p, id); ok && !ev.At.Before(started) {
						engine.Apply(ev)
					}
				})
			}
		}
	}()
	d, e := aula.Open()
	if e != nil {
		return e
	}
	defer d.Close()
	if e = d.PrepareRealtime(); e != nil {
		return e
	}
	renderer := visual.New()
	renderer.Brightness = 65
	defer func() {
		idle := visual.New()
		idle.Brightness = 65
		frameErr := d.SendRealtimeFrame(idle.Render(nil, time.Now()))
		clockErr := d.SetRealtimeClock(time.Now())
		emit(map[string]any{"phase": "restoration", "rgb_write_ok": frameErr == nil, "clock_write_ok": clockErr == nil})
		if err == nil && frameErr != nil {
			err = frameErr
		}
		if err == nil && clockErr != nil {
			err = clockErr
		}
	}()
	trial := lcdtrial.New(time.Now())
	phase := ""
	frames, clockWrites, misses := uint64(0), uint64(0), uint64(0)
	var maxCycle time.Duration
	var clockCycles, clockMisses, rgbOnlyCycles, rgbOnlyMisses uint64
	var maxClockCycle, maxRGBOnlyCycle, maxClockTransaction time.Duration
	var hist [501]uint64
	for {
		cycle := time.Now()
		select {
		case e := <-pipeErrors:
			return fmt.Errorf("hook pipe: %w", e)
		default:
		}
		clock, send, done := trial.Next(cycle, false)
		if done {
			break
		}
		if send {
			clockStart := time.Now()
			if e = d.SetRealtimeClock(clock); e != nil {
				return e
			}
			if took := time.Since(clockStart); took > maxClockTransaction {
				maxClockTransaction = took
			}
			clockWrites++
		}
		if phase != trial.Phase() {
			phase = trial.Phase()
			emit(map[string]any{"phase": phase, "display": clock.Format("15:04:05"), "date": clock.Format("2006-01-02"), "brightness": 65, "fps": 10, "persistence": false})
		}
		now := time.Now()
		frame := renderer.Render(engine.Snapshot(now), now)
		if e = d.SendRealtimeFrame(frame); e != nil {
			return e
		}
		frames++
		elapsed := time.Since(cycle)
		if send {
			clockCycles++
			if elapsed > maxClockCycle {
				maxClockCycle = elapsed
			}
			if elapsed > 100*time.Millisecond {
				clockMisses++
			}
		} else {
			rgbOnlyCycles++
			if elapsed > maxRGBOnlyCycle {
				maxRGBOnlyCycle = elapsed
			}
			if elapsed > 100*time.Millisecond {
				rgbOnlyMisses++
			}
		}
		if elapsed > maxCycle {
			maxCycle = elapsed
		}
		if elapsed > 100*time.Millisecond {
			misses++
		}
		bucket := int((elapsed + time.Millisecond - 1) / time.Millisecond)
		if bucket > 500 {
			bucket = 500
		}
		hist[bucket]++
		if elapsed > 500*time.Millisecond {
			return fmt.Errorf("USB cycle exceeded abort budget")
		}
		if rest := 100*time.Millisecond - elapsed; rest > 0 {
			time.Sleep(rest)
		}
	}
	var cumulative uint64
	p95 := 0
	for ms, count := range hist {
		cumulative += count
		if cumulative*100 >= frames*95 {
			p95 = ms
			break
		}
	}
	s := d.RealtimeStats()
	emit(map[string]any{"phase": "done", "frames": frames, "clock_transactions": clockWrites, "frame_budget_misses": misses, "max_cycle_ms": float64(maxCycle) / float64(time.Millisecond), "p95_cycle_ms": p95, "rgb_apply_errors": s.ApplyErrors, "rgb_apply_recoveries": s.ApplyRecoveries, "one_recent_value_in_ram": true, "history_written": false, "pixel_interface_opened": false, "flash_command_sent": false,
		"clock_cycles": clockCycles, "clock_budget_misses": clockMisses, "rgb_only_cycles": rgbOnlyCycles, "rgb_only_budget_misses": rgbOnlyMisses,
		"max_clock_cycle_ms": float64(maxClockCycle) / float64(time.Millisecond), "max_rgb_only_cycle_ms": float64(maxRGBOnlyCycle) / float64(time.Millisecond), "max_clock_transaction_ms": float64(maxClockTransaction) / float64(time.Millisecond)})
	return nil
}

func main() {
	if err := run(); err != nil {
		emit(map[string]any{"phase": "failed", "error": err.Error()})
		os.Exit(1)
	}
}
