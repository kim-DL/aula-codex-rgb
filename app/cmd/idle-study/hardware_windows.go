//go:build windows

package main

import (
	"aula-codex-rgb-lab/internal/instance"
	"aula-codex-rgb-lab/internal/visual"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	"github.com/parsiya/f108-pro/pkg/aula"
)

// No persistent configuration functions are called here.
func runHardware(args []string) {
	flags := flag.NewFlagSet("hardware-b", flag.ExitOnError)
	refinedSequence := flags.Bool("refined-sequence", false, "20s refined pearl followed by 20s refined ripple")
	sequence := flags.Bool("white-sequence", false, "20s sand pearl then 20s sand ripple")
	allOff := flags.Bool("all-off", false, "send zero to all 104 main-key LEDs")
	keypadOff := flags.Bool("keypad-off", false, "static sand with the four specified keypad LEDs off")
	keypadDiagnostic := flags.Bool("keypad-diagnostic", false, "20s off/keypad red/blue/green/static sand")
	diagnostic := flags.Bool("diagnostic", false, "20s full/off/left/right/full volatile diagnostic")
	variant := flags.String("variant", "pulse", "pulse or pulse-visible")
	seconds := flags.Int("seconds", 40, "seconds of the B candidate")
	output := flags.String("output", "hardware-b-result.json", "evidence file")
	flags.Parse(args)
	if *variant != "pulse" && *variant != "pulse-visible" && *variant != "pulse-defined" && *variant != "pulse-balanced" && *variant != "sand-pearl" && *variant != "sand-ripple" && *variant != "sand-pearl-refined" && *variant != "sand-ripple-refined" {
		panic("unsupported variant")
	}
	if *seconds < 1 || *seconds > 60 {
		panic("seconds must be 1..60")
	}
	result := map[string]any{"started": time.Now().UTC(), "candidate": "B: sand living grain", "variant": *variant, "diagnostic": *diagnostic, "white_sequence": *sequence, "refined_sequence": *refinedSequence, "keypad_diagnostic": *keypadDiagnostic, "keypad_off": *keypadOff, "all_off": *allOff, "brightness": 65, "fps": 10, "persistent_commands_requested": 0, "success": false, "idle_restored": false}
	exitErr := func() error {
		release, ok, err := instance.Acquire()
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("RGB app is running; no USB handle opened")
		}
		defer release()
		d, err := aula.Open()
		if err != nil {
			return err
		}
		defer d.Close()
		idle := visual.New()
		idle.Brightness = 65
		idle.IdleStatic = true
		rest := idle.Render(nil, time.Now())
		var times []float64
		unique := map[string]bool{}
		low, high := [3]uint8{255, 255, 255}, [3]uint8{}
		samples := []map[string]any{}
		misses, count := 0, 0
		defer func() {
			restoreErr := d.SendRealtimeFrame(rest)
			result["idle_restored"] = restoreErr == nil
			if restoreErr != nil {
				result["restore_error"] = restoreErr.Error()
			}
			st := d.RealtimeStats()
			result["initialization_ms"] = float64(st.InitializationNanos) / 1e6
			result["apply_attempt_errors"] = st.ApplyErrors
			result["apply_recovered_frames"] = st.ApplyRecoveries
			result["unique_sent_frame_payloads"] = len(unique)
			result["sent_rgb_min"], result["sent_rgb_max"] = low, high
			result["sent_frame_samples"] = samples
			result["frames"] = count
			result["budget_misses"] = misses
			if len(times) > 0 {
				sort.Float64s(times)
				result["max_ms"] = times[len(times)-1]
				result["p95_ms"] = times[(len(times)*95+99)/100-1]
			}
		}()
		if err = d.PrepareRealtime(); err != nil {
			return err
		}
		keys := visual.Layout()
		send := func(frame aula.LEDFrame) error {
			start := time.Now()
			err := d.SendRealtimeFrame(frame)
			elapsed := time.Since(start)
			if err != nil {
				return err
			}
			count++
			wire := make([]byte, 0, 416)
			for _, k := range visual.Layout() {
				c := frame[k.Index]
				wire = append(wire, k.Index, c[0], c[1], c[2])
				for ch, v := range c {
					low[ch] = min(low[ch], v)
					high[ch] = max(high[ch], v)
				}
			}
			sum := sha256.Sum256(wire)
			hash := hex.EncodeToString(sum[:])
			unique[hash] = true
			if count%10 == 0 {
				samples = append(samples, map[string]any{"frame": count, "hash": hash, "a": frame[aula.KeyNameToIndex["a"]], "h": frame[aula.KeyNameToIndex["h"]], "enter": frame[aula.KeyNameToIndex["enter"]], "f1": frame[aula.KeyNameToIndex["f1"]], "f12": frame[aula.KeyNameToIndex["f12"]], "numlock": frame[aula.KeyNameToIndex["numlock"]], "numslash": frame[aula.KeyNameToIndex["numslash"]], "num7": frame[aula.KeyNameToIndex["num7"]], "num8": frame[aula.KeyNameToIndex["num8"]]})
			}
			ms := float64(elapsed.Microseconds()) / 1000
			times = append(times, ms)
			if count == 1 {
				result["first_frame_ms"] = ms
			}
			if elapsed > 100*time.Millisecond {
				misses++
			}
			if remaining := 100*time.Millisecond - time.Since(start); remaining > 0 {
				time.Sleep(remaining)
			}
			return nil
		}
		frameAt := func(sec, weight float64) aula.LEDFrame {
			if *allOff {
				f := make(aula.LEDFrame, len(keys))
				for _, k := range keys {
					f[k.Index] = [3]uint8{}
				}
				return f
			}
			if *keypadOff {
				f := make(aula.LEDFrame, len(keys))
				for _, k := range keys {
					c := [3]uint8{26, 16, 7}
					if k.Name == "numlock" || k.Name == "numslash" || k.Name == "num7" || k.Name == "num8" {
						c = [3]uint8{}
					}
					f[k.Index] = c
				}
				return f
			}
			if *keypadDiagnostic {
				f := make(aula.LEDFrame, len(keys))
				for _, k := range keys {
					c := [3]uint8{}
					if sec >= 16 {
						c = [3]uint8{26, 16, 7}
					} else if sec >= 4 && (k.Name == "numlock" || k.Name == "numslash" || k.Name == "num7" || k.Name == "num8") {
						if sec < 8 {
							c = [3]uint8{120, 0, 0}
						} else if sec < 12 {
							c = [3]uint8{0, 0, 120}
						} else {
							c = [3]uint8{0, 90, 0}
						}
					}
					f[k.Index] = c
				}
				return f
			}
			if *diagnostic {
				f := make(aula.LEDFrame, len(keys))
				for _, k := range keys {
					c := [3]uint8{26, 16, 7}
					x := k.X + k.W/2
					if (sec >= 4 && sec < 8) || (sec >= 8 && sec < 12 && x >= 11.75) || (sec >= 12 && sec < 16 && x < 11.75) {
						c = [3]uint8{}
					}
					f[k.Index] = c
				}
				return f
			}
			frame := make(aula.LEDFrame, len(keys))
			for _, k := range keys {
				chosen, at := *variant, sec
				if *sequence {
					chosen = "sand-pearl"
					if sec >= 20 {
						chosen = "sand-ripple"
						at = sec - 20
					}
				}
				if *refinedSequence {
					chosen = "sand-pearl-refined"
					at = sec
					if sec >= 20 {
						chosen = "sand-ripple-refined"
						at = sec - 20
					}
				}
				c := candidateColor(chosen, k, at)
				if (*sequence || *refinedSequence) && sec > 19 && sec < 21 {
					envelope := math.Abs(sec - 20)
					for ch, v := range c {
						c[ch] = uint8(math.Round(float64([3]uint8{26, 16, 7}[ch])*(1-envelope) + float64(v)*envelope))
					}
				}
				for ch, v := range c {
					c[ch] = uint8(float64(rest[k.Index][ch])*(1-weight) + float64(v)*weight + .5)
				}
				frame[k.Index] = c
			}
			return frame
		}
		fadeStart := time.Now()
		for elapsed := time.Since(fadeStart).Seconds(); elapsed < 1.8; elapsed = time.Since(fadeStart).Seconds() {
			p := elapsed / 1.8
			weight := p * p * (3 - 2*p)
			if err = send(frameAt(0, weight)); err != nil {
				return err
			}
		}
		sceneStart := time.Now()
		sceneFrames := 0
		result["scene_started"] = sceneStart.UTC()
		for elapsed := time.Since(sceneStart).Seconds(); elapsed < float64(*seconds); elapsed = time.Since(sceneStart).Seconds() {
			if err = send(frameAt(elapsed, 1)); err != nil {
				return err
			}
			sceneFrames++
		}
		sceneElapsed := time.Since(sceneStart)
		result["scene_seconds"] = sceneElapsed.Seconds()
		result["scene_frames"] = sceneFrames
		result["scene_average_fps"] = float64(sceneFrames) / sceneElapsed.Seconds()
		fadeStart = time.Now()
		for elapsed := time.Since(fadeStart).Seconds(); elapsed < 1.8; elapsed = time.Since(fadeStart).Seconds() {
			p := elapsed / 1.8
			weight := 1 - p*p*(3-2*p)
			if err = send(frameAt(float64(*seconds)+elapsed, weight)); err != nil {
				return err
			}
		}
		result["success"] = true
		return nil
	}()
	if exitErr != nil {
		result["error"] = exitErr.Error()
	}
	result["finished"] = time.Now().UTC()
	data, _ := json.MarshalIndent(result, "", "  ")
	if err := os.WriteFile(*output, data, 0600); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(data))
	if exitErr != nil {
		os.Exit(1)
	}
}
