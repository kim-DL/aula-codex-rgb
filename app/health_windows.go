//go:build windows && rgb_gui

package main

import (
	"aula-codex-rgb-lab/internal/status"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type taskHealth struct {
	Thread           string      `json:"thread"`
	Turn             string      `json:"turn"`
	State            status.Kind `json:"state"`
	LastEvent        string      `json:"last_event"`
	Source           string      `json:"source"`
	Stale            bool        `json:"stale"`
	ActiveTools      int         `json:"active_tools"`
	PendingApprovals int         `json:"pending_approvals"`
}

type usbHealth struct {
	IdleEffect              string `json:"idle_effect"`
	mu                      sync.Mutex
	Started                 time.Time    `json:"started"`
	Updated                 time.Time    `json:"updated"`
	PID                     int          `json:"pid"`
	Connected               bool         `json:"connected"`
	Frames                  uint64       `json:"frames"`
	SendErrors              uint64       `json:"send_errors"`
	ApplyErrors             uint64       `json:"apply_attempt_errors"`
	ApplyRecoveries         uint64       `json:"apply_recovered_frames"`
	OpenAttempts            uint64       `json:"open_attempts"`
	OpenErrors              uint64       `json:"open_errors"`
	InitializationAttempts  uint64       `json:"initialization_attempts"`
	InitializationErrors    uint64       `json:"initialization_errors"`
	InitializationNanos     uint64       `json:"initialization_nanos"`
	LastInitializationNanos uint64       `json:"last_initialization_nanos"`
	Connections             uint64       `json:"connections"`
	ScheduleGaps            uint64       `json:"schedule_gaps"`
	LastFrame               time.Time    `json:"last_frame"`
	LastError               time.Time    `json:"last_error"`
	LastConnection          time.Time    `json:"last_connection"`
	ConsecutiveFailures     int          `json:"consecutive_failures"`
	TransferNanos           uint64       `json:"transfer_nanos"`
	MaxTransferNanos        uint64       `json:"max_transfer_nanos"`
	Brightness              uint32       `json:"brightness"`
	TargetFPS               int32        `json:"target_fps"`
	Paused                  bool         `json:"paused"`
	HeapAlloc               uint64       `json:"go_heap_alloc"`
	HeapSys                 uint64       `json:"go_heap_sys"`
	NumGC                   uint32       `json:"go_num_gc"`
	Goroutines              int          `json:"goroutines"`
	TransferHistogram       [256]uint64  `json:"transfer_histogram_ms"` // ceil(ms), last bucket >=255ms
	BudgetMisses            uint64       `json:"frame_budget_misses"`
	Effect                  string       `json:"effect"`
	Thinking                string       `json:"thinking_effect"`
	ConnectionsStyle        string       `json:"connections_style"`
	MainState               status.Kind  `json:"main_state"`
	TemporaryIgnored        uint64       `json:"temporary_events_ignored"`
	LastFrameState          status.Kind  `json:"last_frame_state"`
	IdleFrames              uint64       `json:"idle_frames"`
	LastIdleFrame           time.Time    `json:"last_idle_frame"`
	IdleRGBMin              [3]uint8     `json:"idle_rgb_min"`
	IdleRGBMax              [3]uint8     `json:"idle_rgb_max"`
	Tasks                   []taskHealth `json:"tasks"`
}

func (a *application) writeHealth() {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	tasks := a.engine.Snapshot(time.Now())
	ignored := a.engine.IgnoredTemporaryEvents()
	a.health.mu.Lock()
	a.health.TemporaryIgnored = ignored
	a.health.MainState = status.Idle
	a.health.Tasks = make([]taskHealth, 0, len(tasks))
	for _, task := range tasks {
		a.health.Tasks = append(a.health.Tasks, taskHealth{task.Thread, task.Turn, task.State, task.Event, task.Source, task.Stale, task.ActiveTools, task.PendingApprovals})
	}
	if len(tasks) > 0 {
		a.health.MainState = tasks[0].State
	}
	a.health.Started, a.health.Updated, a.health.PID = a.startedAt, time.Now().UTC(), os.Getpid()
	a.health.Brightness, a.health.TargetFPS, a.health.Paused = a.brightness.Load(), a.fps.Load(), a.paused.Load()
	a.health.Effect, a.health.Thinking = "focus", "living"
	a.health.ConnectionsStyle = "flow"
	a.health.IdleEffect = "sand_white_drift"
	a.health.HeapAlloc, a.health.HeapSys, a.health.NumGC, a.health.Goroutines = mem.HeapAlloc, mem.HeapSys, mem.NumGC, runtime.NumGoroutine()
	b, err := json.Marshal(&a.health)
	a.health.mu.Unlock()
	if err == nil {
		_ = atomicWrite(filepath.Join(a.dataDir, "health.json"), b, 0600)
	}
}

func (a *application) hardwareEvent(kind string, err error, metadata ...map[string]any) {
	detail := ""
	if err != nil {
		detail = shortError(err)
	}
	row := map[string]any{"at": time.Now().UTC(), "kind": kind, "error": err != nil, "detail": detail}
	for _, fields := range metadata {
		for k, v := range fields {
			row[k] = v
		}
	}
	b, marshalErr := json.Marshal(row)
	if marshalErr != nil {
		return
	}
	f, openErr := os.OpenFile(filepath.Join(a.dataDir, "hardware-events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if openErr != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(b, '\n'))
}
