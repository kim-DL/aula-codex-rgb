// Demo is built on demand for physical validation; never linked into the app.
// No USB writes happen without --hardware. Stop the tray app before running it.
package main

import (
	"aula-codex-rgb-lab/internal/instance"
	"aula-codex-rgb-lab/internal/status"
	"aula-codex-rgb-lab/internal/visual"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/parsiya/f108-pro/pkg/aula"
	"math"
	"os"
	"sort"
	"strconv"
	"time"
)

func main() {
	hardware := flag.Bool("hardware", false, "explicitly send RGB frames")
	seconds := flag.Int("seconds", 4, "seconds per state")
	restDemo := flag.Bool("rest", false, "completion 8s, calm idle 40s, tool 3s, idle 6s")
	lazyInit := flag.Bool("lazy-init", false, "diagnostic baseline: initialize inside the first timed frame")
	thinkingOnly := flag.Bool("thinking-only", false, "only the current thinking animation")
	latencyCheck := flag.Bool("latency-check", false, "only the existing thinking and tool effects")
	output := flag.String("output", "demo-result.json", "evidence file")
	flag.Parse()
	if !*hardware || *seconds < 1 || *seconds > 60 {
		fmt.Println("demo --hardware --seconds 1..60 --output file")
		os.Exit(2)
	}
	release, ok, err := instance.Acquire()
	if err != nil || !ok {
		panic("stop RGB app before hardware demo")
	}
	defer release()
	d, err := aula.Open()
	if err != nil {
		panic(err)
	}
	r := visual.New()
	r.Brightness = 65
	result := map[string]any{"brightness": 65, "fps": 10, "flash_command_sent": false}
	var latencies []float64
	samples := []map[string]any{}
	stages := []map[string]any{}
	count, misses := 0, 0
	defer func() {
		idle := visual.New()
		idle.Brightness = 65
		restore := d.SendRealtimeFrame(idle.Render(nil, time.Now()))
		stats := d.RealtimeStats()
		d.Close()
		result["idle_restored"] = restore == nil
		result["apply_attempt_errors"] = stats.ApplyErrors
		result["apply_recovered_frames"] = stats.ApplyRecoveries
		result["initialization_ms"] = float64(stats.InitializationNanos) / 1e6
		result["initialization_attempts"] = stats.InitializationAttempts
		result["lazy_initialization"] = *lazyInit
		result["timing_samples"] = samples
		result["frames"] = count
		result["budget_misses"] = misses
		result["stages"] = stages
		if len(latencies) > 0 {
			sort.Float64s(latencies)
			result["p95_ms"] = latencies[int(math.Ceil(.95*float64(len(latencies))))-1]
			result["max_ms"] = latencies[len(latencies)-1]
		}
		b, _ := json.MarshalIndent(result, "", "  ")
		_ = os.WriteFile(*output, b, 0600)
		fmt.Println(string(b))
	}()
	if !*lazyInit {
		if err = d.PrepareRealtime(); err != nil {
			result["error"] = err.Error()
			return
		}
		// Keep all 200ms firmware settling time outside the animation timeline.
		r.Start = time.Now()
	}
	all := []status.Kind{status.Thinking, status.Tool, status.Coding, status.Editing, status.Responding, status.Approval, status.Failed, status.Complete, status.Idle}
	if *latencyCheck {
		all = []status.Kind{status.Thinking, status.Tool}
	}
	if *thinkingOnly {
		all = []status.Kind{status.Thinking}
	}
	durations := make([]int, len(all))
	for i := range durations {
		durations[i] = *seconds
	}
	if *restDemo {
		all = []status.Kind{status.Complete, status.Idle, status.Tool, status.Idle}
		durations = []int{8, 40, 3, 6}
	}
	started := time.Now()
	for stage, state := range all {
		at := time.Now()
		fmt.Printf("%s stage %d/%d %s %ds\n", at.Format(time.RFC3339), stage+1, len(all), state, durations[stage])
		lo, hi := map[string]int{}, map[string]int{}
		for n := 1; n <= 12; n++ {
			lo["f"+strconv.Itoa(n)] = 255
		}
		frames := 0
		for time.Since(at) < time.Duration(durations[stage])*time.Second {
			now := time.Now()
			tasks := []status.Task{{Thread: "physical-demo", State: state, Updated: at}}
			if state == status.Tool {
				tasks[0].Tool, tasks[0].ActiveTools = "exec", 1
			}
			if state == status.Idle {
				tasks = nil
			}
			renderStart := time.Now()
			frame := r.Render(tasks, now)
			renderElapsed := time.Since(renderStart)
			start := time.Now()
			before := d.RealtimeStats()
			if err = d.SendRealtimeFrame(frame); err != nil {
				result["error"] = err.Error()
				return
			}
			elapsed := time.Since(start)
			after := d.RealtimeStats()
			if count < 5 || elapsed > 100*time.Millisecond {
				samples = append(samples, map[string]any{"state": state, "frame": count, "render_ms": float64(renderElapsed.Microseconds()) / 1000, "send_ms": float64(elapsed.Microseconds()) / 1000, "initialization_in_send_ms": float64(after.InitializationNanos-before.InitializationNanos) / 1e6, "timing": after.LastFrame})
			}
			if count == 0 {
				result["first_frame_ms"] = float64(elapsed.Microseconds()) / 1000
			}
			latencies = append(latencies, float64(elapsed.Microseconds())/1000)
			if elapsed > 100*time.Millisecond {
				misses++
			}
			frames++
			count++
			for n := 1; n <= 12; n++ {
				name := "f" + strconv.Itoa(n)
				c := frame[aula.KeyNameToIndex[name]]
				peak := max(int(c[0]), int(c[1]), int(c[2]))
				lo[name] = min(lo[name], peak)
				hi[name] = max(hi[name], peak)
			}
			if rest := 100*time.Millisecond - time.Since(start); rest > 0 {
				time.Sleep(rest)
			}
		}
		stages = append(stages, map[string]any{"state": state, "started_at": at, "ended_at": time.Now(), "frames": frames, "f_low": lo, "f_high": hi})
		fmt.Println(state, "done")
	}
	result["success"] = true
	result["seconds"] = time.Since(started).Seconds()
	result["average_fps"] = float64(count) / time.Since(started).Seconds()
}
