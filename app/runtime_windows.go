//go:build windows && rgb_gui

package main

import (
	"aula-codex-rgb-lab/internal/bridge"
	"aula-codex-rgb-lab/internal/events"
	"aula-codex-rgb-lab/internal/instance"
	"aula-codex-rgb-lab/internal/status"
	"aula-codex-rgb-lab/internal/trayicon"
	"aula-codex-rgb-lab/internal/visual"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/getlantern/systray"
	"github.com/parsiya/f108-pro/pkg/aula"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

type application struct {
	engine     *status.Engine
	renderer   visual.Renderer
	fps        atomic.Int32
	brightness atomic.Uint32
	paused     atomic.Bool
	state      atomic.Value
	ctx        context.Context
	cancel     context.CancelFunc
	sessions   string
	config     string
	dataDir    string
	statusFile string
	startedAt  time.Time
	live       *aula.Device // Owned exclusively by hardwareLoop.
	health     usbHealth
}

var errAlreadyRunning = errors.New("AULA RGB is already running")

func savePrefs(a *application) {
	p := prefs{Brightness: uint8(a.brightness.Load()), FPS: targetFPS, Paused: a.paused.Load()}
	b, _ := json.MarshalIndent(p, "", "  ")
	_ = os.WriteFile(a.config, b, 0600)
}
func newApp() (*application, error) {
	dir, err := userData()
	if err != nil {
		return nil, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	p := loadPrefs(filepath.Join(dir, "settings.json"))
	ctx, cancel := context.WithCancel(context.Background())
	a := &application{engine: status.New(), renderer: visual.New(), ctx: ctx, cancel: cancel,
		sessions: filepath.Join(home, ".codex", "sessions"), config: filepath.Join(dir, "settings.json"),
		dataDir: dir, statusFile: filepath.Join(dir, "status.json"), startedAt: time.Now().UTC()}
	a.fps.Store(int32(p.FPS))
	a.brightness.Store(uint32(p.Brightness))
	a.paused.Store(p.Paused)
	a.renderer.Brightness = p.Brightness
	_ = os.Remove(filepath.Join(dir, "control.json"))
	a.state.Store("")
	a.setState("starting")
	savePrefs(a)
	a.writeHealth()
	return a, nil
}

func runApp() error {
	release, acquired, err := instance.Acquire()
	if err != nil {
		return err
	}
	if !acquired {
		return errAlreadyRunning
	}
	defer release()
	a, err := newApp()
	if err != nil {
		return err
	}
	beforePriority, afterPriority, priorityErr := normalizeScheduling()
	a.hardwareEvent("scheduling_priority", priorityErr, map[string]any{"before": beforePriority, "after": afterPriority})
	pipeErrors := make(chan error, 1)
	a.engine.Clock().SetEnabled(desktopRunning())
	go func() {
		if err := bridge.Serve(a.ctx, a.engine, pipeErrors); err != nil && a.ctx.Err() == nil {
			select {
			case pipeErrors <- err:
			default:
			}
		}
	}()
	go a.tailRollouts()
	go a.desktopLoop()
	go a.controlLoop()
	go a.heartbeatLoop()
	hardwareDone := make(chan struct{})
	go func() { defer close(hardwareDone); a.hardwareLoop() }()
	systray.Run(func() { a.trayReady(pipeErrors) }, func() { a.cancel(); savePrefs(a) })
	a.cancel()
	<-hardwareDone
	a.setState("stopped")
	a.writeHealth()
	return nil
}

func (a *application) setState(value string) {
	old, _ := a.state.Load().(string)
	if old == value {
		return
	}
	a.state.Store(value)
	b, _ := json.Marshal(appStatus{State: value, Updated: time.Now().UTC()})
	_ = atomicWrite(a.statusFile, b, 0600)
}

func (a *application) heartbeatLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			state, _ := a.state.Load().(string)
			b, _ := json.Marshal(appStatus{State: state, Updated: time.Now().UTC()})
			_ = atomicWrite(a.statusFile, b, 0600)
			a.writeHealth()
		}
	}
}

func (a *application) tailRollouts() {
	tail := events.NewTailer(a.sessions)
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			for _, task := range a.engine.Snapshot(time.Now()) {
				tail.WatchThread(task.Thread)
			}
			_ = tail.Poll(func(b []byte, path, sessionID string) {
				if e, ok := events.DecodeForSession(b, path, sessionID); ok {
					if !e.At.Before(a.startedAt) {
						accepted := a.engine.Apply(e)
						if !accepted {
							a.engine.RecoverClockStart(e)
						}
						events.AppendMetadata(e, accepted)
					} else {
						a.engine.RecoverClockStart(e)
					}
				}
			})
		}
	}
}

func (a *application) controlLoop() {
	started := a.startedAt.Add(-time.Second)
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			b, err := os.ReadFile(filepath.Join(a.dataDir, "control.json"))
			if err != nil {
				continue
			}
			var signal controlSignal
			if json.Unmarshal(b, &signal) != nil || signal.At.IsZero() || !signal.At.After(started) {
				continue
			}
			switch signal.Action {
			case "pause":
				a.paused.Store(true)
				savePrefs(a)
				a.setState("일시 정지")
			case "resume":
				a.paused.Store(false)
				savePrefs(a)
				a.setState("재개 중")
			case "stop":
				a.cancel()
				systray.Quit()
				return
			}

			started = signal.At
		}
	}
}

func (a *application) hardwareLoop() {
	var retry time.Time
	var lastIteration time.Time
	lastSignature := ""
	var changedAt time.Time
	settled := false
	failures := 0
	var lastSend time.Time
	var lastIdleFrame aula.LEDFrame
	lastTaskCount := -1
	lastPaused := false
	animationStarted := false
	clockForce := false
	defer func() {
		if a.live != nil {
			// Restore the PC clock on a normal shutdown, with the same HID owner.
			if err := a.live.SetRealtimeClock(time.Now()); err != nil {
				a.hardwareEvent("clock_shutdown_failed", err)
			}
			_ = a.sendIdle()
			a.live.Close()
			a.live = nil
		}
		a.health.mu.Lock()
		a.health.Connected = false
		a.health.mu.Unlock()
	}()
	for a.ctx.Err() == nil {
		now := time.Now()
		if scheduleGap(lastIteration, now) {
			if a.live != nil {
				a.live.Close()
				a.live = nil
			}
			retry = time.Time{}
			a.health.mu.Lock()
			a.health.Connected = false
			a.health.ScheduleGaps++
			a.health.mu.Unlock()
			a.hardwareEvent("schedule_gap_reopen", nil)
		}
		lastIteration = now
		if a.live == nil {
			if time.Now().Before(retry) {
				if !sleepContext(a.ctx, 150*time.Millisecond) {
					return
				}
				continue
			}
			a.health.mu.Lock()
			a.health.OpenAttempts++
			a.health.mu.Unlock()
			d, err := aula.Open()
			if err != nil {
				failures++
				a.health.mu.Lock()
				a.health.OpenErrors++
				a.health.ConsecutiveFailures = failures
				a.health.LastError = time.Now().UTC()
				a.health.mu.Unlock()
				a.hardwareEvent("open_failed", err)
				delay := reconnectDelay(failures)
				a.setState(fmt.Sprintf("USB 연결 대기 · %d초 후 재확인", int(delay.Seconds())))
				retry = time.Now().Add(delay)
				continue
			}
			a.setState("USB 연결됨 · 장치 초기 준비")
			prepareErr := d.PrepareRealtime()
			preparation := d.RealtimeStats()
			a.health.mu.Lock()
			a.health.InitializationAttempts += preparation.InitializationAttempts
			a.health.InitializationNanos += preparation.InitializationNanos
			a.health.LastInitializationNanos = preparation.InitializationNanos
			if prepareErr != nil {
				a.health.InitializationErrors++
				a.health.LastError = time.Now().UTC()
			}
			a.health.mu.Unlock()
			if prepareErr != nil {
				d.Close()
				failures++
				a.health.mu.Lock()
				a.health.ConsecutiveFailures = failures
				a.health.mu.Unlock()
				a.hardwareEvent("prepare_failed", prepareErr)
				retry = time.Now().Add(reconnectDelay(failures))
				a.setState("USB 초기 준비 실패 · 재연결 대기")
				continue
			}
			a.hardwareEvent("prepared", nil, map[string]any{"initialization_ms": float64(preparation.InitializationNanos) / 1e6})
			a.live = d
			clockForce = true
			if !animationStarted {
				a.renderer.Start = time.Now()
				animationStarted = true
			}
			a.health.mu.Lock()
			a.health.Connected = true
			a.health.Connections++
			a.health.LastConnection = time.Now().UTC()
			a.health.mu.Unlock()
			a.hardwareEvent("opened", nil)
			lastTaskCount = -1
			lastSend = time.Time{}
			lastIdleFrame = nil
			lastSignature = ""
			a.setState("USB 연결됨 · 실시간 조명 준비")
		}
		cycleStart := time.Now()
		now = cycleStart
		if plan, needed := a.engine.Clock().Next(now, clockForce); needed {
			err := a.live.SetRealtimeClock(plan.Time)
			a.engine.Clock().Commit(plan, time.Now(), err)
			clockForce = false
			if err != nil {
				// A failed transaction may be incomplete: reconnect before any RGB
				// writes. Clock retries have their own bounded backoff/circuit breaker.
				a.hardwareEvent("clock_sync_failed", err)
				a.live.Close()
				a.live = nil
				failures++
				a.health.mu.Lock()
				a.health.Connected = false
				a.health.SendErrors++
				a.health.LastError = time.Now().UTC()
				a.health.mu.Unlock()
				retry = time.Now().Add(reconnectDelay(failures))
				continue
			}
		}
		a.renderer.Brightness = uint8(a.brightness.Load())
		a.renderer.IdleStatic = a.paused.Load()
		tasks := a.engine.Snapshot(now)
		if a.paused.Load() {
			tasks = nil
		}
		frame := a.renderer.Render(tasks, now)
		// Evaluate the slow motion at 10Hz, send changed idle frames promptly,
		// and retain the 1Hz keepalive for identical quantized/paused frames.
		if len(tasks) == 0 && !lastSend.IsZero() && now.Sub(lastSend) < time.Second && maps.Equal(frame, lastIdleFrame) {
			if delay := time.Second/targetFPS - time.Since(cycleStart); delay > 0 && !sleepContext(a.ctx, delay) {
				return
			}
			continue
		}
		start := time.Now()
		before := a.live.RealtimeStats()
		sendErr := a.live.SendRealtimeFrame(frame)
		after := a.live.RealtimeStats()
		if after.LastFrame.TotalNanos > uint64(time.Second/targetFPS) {
			kind := status.Idle
			if len(tasks) > 0 {
				kind = tasks[0].State
			}
			a.hardwareEvent("slow_frame", sendErr, map[string]any{"state": kind, "timing": after.LastFrame, "render_ms": float64(start.Sub(now).Microseconds()) / 1000})
		}
		a.health.mu.Lock()
		a.health.ApplyErrors += after.ApplyErrors - before.ApplyErrors
		a.health.ApplyRecoveries += after.ApplyRecoveries - before.ApplyRecoveries
		if after.ApplyErrors > before.ApplyErrors {
			a.health.LastError = time.Now().UTC()
		}
		a.health.mu.Unlock()
		if after.ApplyRecoveries > before.ApplyRecoveries {
			a.hardwareEvent("apply_recovered", nil, map[string]any{"windows_error_code": 31, "attempt_errors": after.ApplyErrors - before.ApplyErrors, "frame_transfer_ms": float64(time.Since(start).Microseconds()) / 1000})
		}
		if err := sendErr; err != nil {
			a.setState("USB 전송 오류 · 재연결 시도")
			a.live.Close()
			a.live = nil
			failures++
			a.health.mu.Lock()
			a.health.Connected = false
			a.health.SendErrors++
			a.health.ConsecutiveFailures = failures
			a.health.LastError = time.Now().UTC()
			a.health.mu.Unlock()
			a.hardwareEvent("send_failed", err)
			retry = time.Now().Add(reconnectDelay(failures))
			continue
		}
		failures = 0
		lastSend = time.Now()
		if len(tasks) == 0 {
			if lastIdleFrame == nil {
				lastIdleFrame = make(aula.LEDFrame, len(frame))
			}
			for idx, c := range frame {
				lastIdleFrame[idx] = c
			}
		} else {
			lastIdleFrame = nil
		}
		transfer := uint64(time.Since(start).Nanoseconds())
		a.health.mu.Lock()
		a.health.Frames++
		a.health.LastFrameState = status.Idle
		if len(tasks) > 0 {
			a.health.LastFrameState = tasks[0].State
		} else {
			a.health.IdleFrames++
			a.health.LastIdleFrame = lastSend
			lo, hi := [3]uint8{255, 255, 255}, [3]uint8{}
			for _, color := range frame {
				for ch, value := range color {
					lo[ch] = min(lo[ch], value)
					hi[ch] = max(hi[ch], value)
				}
			}
			a.health.IdleRGBMin, a.health.IdleRGBMax = lo, hi
		}
		a.health.ConsecutiveFailures = 0
		a.health.LastFrame = time.Now().UTC()
		a.health.TransferNanos += transfer
		bucket := (transfer + 999999) / 1000000
		if bucket > 255 {
			bucket = 255
		}
		a.health.TransferHistogram[bucket]++
		if transfer > uint64(time.Second/time.Duration(a.fps.Load())) {
			a.health.BudgetMisses++
		}
		if transfer > a.health.MaxTransferNanos {
			a.health.MaxTransferNanos = transfer
		}
		a.health.mu.Unlock()
		paused := a.paused.Load()
		if lastTaskCount != len(tasks) || lastPaused != paused {
			if paused {
				a.setState("일시 정지")
			} else {
				a.setState(fmt.Sprintf("실시간 RGB · %d fps · 작업 %d개", targetFPS, len(tasks)))
			}
			lastTaskCount, lastPaused = len(tasks), paused
		}
		if events.DiagnosticsEnabled() {
			signature := taskSignature(tasks)
			settleDelay := 600 * time.Millisecond
			if signature != lastSignature {
				lastSignature, changedAt, settled = signature, time.Now(), false
				a.traceLightFrame("transition", tasks, frame, time.Since(start))
			} else if !settled && time.Since(changedAt) >= settleDelay {
				settled = true
				a.traceLightFrame("settled", tasks, frame, time.Since(start))
			}
		}
		interval := time.Second/time.Duration(a.fps.Load()) - time.Since(cycleStart)
		if interval > 0 && !sleepContext(a.ctx, interval) {
			return
		}
	}
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (a *application) sendIdle() error {
	if a.live == nil {
		return nil
	}
	r := visual.New()
	r.Brightness = uint8(a.brightness.Load())
	frame := r.Render(nil, time.Now())
	return a.live.SendRealtimeFrame(frame)
}

func shortError(err error) string {
	s := strings.ReplaceAll(err.Error(), "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 90 {
		s = s[:90]
	}
	return s
}

func shortThread(thread string) string {
	if i := strings.Index(thread, "#subagent:"); i >= 0 {
		thread = "agent-" + thread[i+10:]
	}
	if len(thread) > 8 {
		return thread[:8]
	}
	return thread
}

func statusLabel(k status.Kind) string {
	switch k {
	case status.Thinking:
		return "생각 추정"
	case status.Responding:
		return "응답 기록"
	case status.Coding:
		return "코드 수정"
	case status.Editing:
		return "파일 수정"
	case status.Reading:
		return "문서·자료 읽기"
	case status.Tool:
		return "도구 실행"
	case status.Approval:
		return "승인 대기"
	case status.Complete:
		return "완료"
	case status.Failed:
		return "오류"
	case status.Interrupted:
		return "중단"
	default:
		return "대기"
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] != "run" {
		return
	}
	if err := runApp(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
func (a *application) trayReady(pipeErrors <-chan error) {
	systray.SetIcon(trayicon.Bytes())
	systray.SetTitle("AULA RGB")
	systray.SetTooltip("AULA · 생명 파동 / 집중 · 65% / 10fps")
	state := systray.AddMenuItem("시작 중…", "작업과 USB 상태")
	state.Disable()
	tasksMenu := systray.AddMenuItem("Codex 작업", "확정·추정 상태")
	items := make([]*systray.MenuItem, 11)
	for i := range items {
		items[i] = tasksMenu.AddSubMenuItem("", "")
		items[i].Disable()
		items[i].Hide()
	}
	info := systray.AddMenuItem("생명 파동 · 집중 · 65% · 10fps", "기본 조명")
	info.Disable()
	systray.AddSeparator()
	pause := systray.AddMenuItem("일시 정지 / 재개", "조명 반응을 일시 정지합니다")
	quit := systray.AddMenuItem("종료", "차분한 대기 조명으로 돌아가고 종료합니다")
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		lastState, lastCount := "", -1
		labels := make([]string, len(items))
		for {
			select {
			case <-a.ctx.Done():
				return
			case err := <-pipeErrors:
				if err != nil {
					a.setState("이벤트 연결 오류: " + shortError(err))
				}
			case <-ticker.C:
				current := a.state.Load().(string)
				if current != lastState {
					state.SetTitle(current)
					lastState = current
				}
				tasks := a.engine.Snapshot(time.Now())
				if len(tasks) != lastCount {
					tasksMenu.SetTitle(fmt.Sprintf("Codex 작업 (%d)", len(tasks)))
					lastCount = len(tasks)
				}
				for i, item := range items {
					label := ""
					if i < len(tasks) {
						confidence := "확정"
						if !tasks[i].Confirmed {
							confidence = "추정"
						}
						if tasks[i].Stale {
							confidence += " · 상태 확인 지연"
						}
						label = shortThread(tasks[i].Thread) + " · " + statusLabel(tasks[i].State) + " · " + confidence
					}
					if label != labels[i] {
						if label == "" {
							item.Hide()
						} else {
							item.SetTitle(label)
							item.Show()
						}
						labels[i] = label
					}
				}
			}
		}
	}()
	go func() {
		for {
			select {
			case <-a.ctx.Done():
				systray.Quit()
				return
			case <-pause.ClickedCh:
				a.paused.Store(!a.paused.Load())
				savePrefs(a)
			case <-quit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}
