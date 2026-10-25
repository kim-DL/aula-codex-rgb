//go:build windows

package main

import (
	"aula-codex-rgb-lab/internal/visual"
	"math"
	"testing"
)

func TestIdleStudyCandidatesPreserveDimSandAndSmoothMotion(t *testing.T) {
	for _, variant := range []string{"drift", "pulse"} {
		for _, key := range visual.Layout() {
			lo, hi := 255, 0
			previous := candidateColor(variant, key, 0)
			for n := 1; n <= 1200; n++ {
				c := candidateColor(variant, key, float64(n)/10)
				if c[0] > 28 || c[1] > 18 || c[2] > 7 || c[0] < 20 || c[2] < 5 || c[0] <= c[1] || c[1] <= c[2] {
					t.Fatalf("%s %s invalid sand %v", variant, key.Name, c)
				}
				for ch, v := range c {
					if math.Abs(float64(v)-float64(previous[ch])) > 1 {
						t.Fatalf("%s abrupt 100ms change", variant)
					}
				}
				lo = min(lo, int(c[0]))
				hi = max(hi, int(c[0]))
				previous = c
			}
			if hi-lo < 3 {
				t.Fatalf("%s key %s has no visible-range motion", variant, key.Name)
			}
		}
	}
}
func TestIdleStudyCandidateIsTimeBasedWithoutFrameHistory(t *testing.T) {
	key := visual.Layout()[0]
	want := candidateColor("pulse", key, 37.3)
	for sec := 0.; sec < 37.3; sec += 1. / 15 {
		candidateColor("pulse", key, sec)
	}
	if candidateColor("pulse", key, 37.3) != want {
		t.Fatal("frame-rate dependent")
	}
	if candidateColor("pulse", key, -1) != candidateColor("pulse", key, 0) {
		t.Fatal("negative time")
	}
}
func TestVisibleCandidateKeepsPeakButCreatesMovingSpatialContrast(t *testing.T) {
	keys := visual.Layout()
	minAll, maxAll, maxStep, maxSpread := 255, 0, 0, 0
	for _, key := range keys {
		previous := candidateColor("pulse-visible", key, 0)
		lo, hi := 255, 0
		for n := 1; n <= 1200; n++ {
			c := candidateColor("pulse-visible", key, float64(n)/10)
			if c[0] > 28 || c[1] > 18 || c[2] > 7 || c[0] < 10 || c[2] < 2 || c[0] <= c[1] || c[1] <= c[2] {
				t.Fatalf("invalid visible sand %v", c)
			}
			for ch, v := range c {
				maxStep = max(maxStep, int(math.Abs(float64(v)-float64(previous[ch]))))
			}
			lo = min(lo, int(c[0]))
			hi = max(hi, int(c[0]))
			previous = c
		}
		minAll = min(minAll, lo)
		maxAll = max(maxAll, hi)
		if hi-lo < 5 {
			t.Fatalf("insufficient moving range for %s: %d", key.Name, hi-lo)
		}
	}
	for n := 0; n < 400; n++ {
		lo, hi := 255, 0
		for _, k := range keys {
			c := candidateColor("pulse-visible", k, float64(n)/10)
			lo = min(lo, int(c[0]))
			hi = max(hi, int(c[0]))
		}
		maxSpread = max(maxSpread, hi-lo)
	}
	if maxStep > 2 || maxSpread < 12 {
		t.Fatalf("poor visibility/smoothness: step=%d spread=%d", maxStep, maxSpread)
	}
	t.Logf("R signal=%d..%d, max 100ms channel step=%d, simultaneous spatial R spread=%d", minAll, maxAll, maxStep, maxSpread)
}
func TestDefinedCandidateHasReadableMovingBandWithoutHigherPeak(t *testing.T) {
	keys := visual.Layout()
	maxStep, maxSpread := 0, 0
	for _, key := range keys {
		previous := candidateColor("pulse-defined", key, 0)
		lo, hi := 255, 0
		for n := 1; n <= 600; n++ {
			c := candidateColor("pulse-defined", key, float64(n)/10)
			if c[0] > 28 || c[1] > 18 || c[2] > 7 || c[0] < 3 || c[0] <= c[1] || c[1] < c[2] {
				t.Fatalf("invalid sand %v", c)
			}
			for ch, v := range c {
				maxStep = max(maxStep, int(math.Abs(float64(v)-float64(previous[ch]))))
			}
			lo = min(lo, int(c[0]))
			hi = max(hi, int(c[0]))
			previous = c
		}
		if hi-lo < 15 {
			t.Fatalf("no clear travel on %s, range=%d", key.Name, hi-lo)
		}
	}
	for n := 0; n < 400; n++ {
		lo, hi := 255, 0
		for _, k := range keys {
			c := candidateColor("pulse-defined", k, float64(n)/10)
			lo = min(lo, int(c[0]))
			hi = max(hi, int(c[0]))
		}
		maxSpread = max(maxSpread, hi-lo)
	}
	if maxStep > 3 || maxSpread < 20 {
		t.Fatalf("step=%d spread=%d", maxStep, maxSpread)
	}
	t.Logf("max channel step/100ms=%d, max simultaneous R contrast=%d", maxStep, maxSpread)
}
func TestBalancedCandidateKeepsSandFloorAndStrongerSoftHighlights(t *testing.T) {
	keys := visual.Layout()
	maxStep, maxSpread := 0, 0
	loAll, hiAll := [3]uint8{255, 255, 255}, [3]uint8{}
	for _, key := range keys {
		previous := candidateColor("pulse-balanced", key, 0)
		lo, hi := 255, 0
		for n := 1; n <= 1200; n++ {
			c := candidateColor("pulse-balanced", key, float64(n)/10)
			if c[0] > 57 || c[1] > 36 || c[2] > 15 || c[0] < 24 || c[1] < 15 || c[2] < 6 || c[0] <= c[1] || c[1] <= c[2] {
				t.Fatalf("invalid balanced sand %v", c)
			}
			for ch, v := range c {
				maxStep = max(maxStep, int(math.Abs(float64(v)-float64(previous[ch]))))
				loAll[ch] = min(loAll[ch], v)
				hiAll[ch] = max(hiAll[ch], v)
			}
			lo = min(lo, int(c[0]))
			hi = max(hi, int(c[0]))
			previous = c
		}
		if hi-lo < 12 {
			t.Fatalf("insufficient range for %s: %d", key.Name, hi-lo)
		}
	}
	for n := 0; n < 600; n++ {
		lo, hi := 255, 0
		for _, k := range keys {
			c := candidateColor("pulse-balanced", k, float64(n)/10)
			lo = min(lo, int(c[0]))
			hi = max(hi, int(c[0]))
		}
		maxSpread = max(maxSpread, hi-lo)
	}
	if maxStep > 3 || maxSpread < 20 {
		t.Fatalf("step=%d spread=%d", maxStep, maxSpread)
	}
	t.Logf("RGB=%v..%v, max channel step/100ms=%d, max spatial R contrast=%d", loAll, hiAll, maxStep, maxSpread)
}
func TestSandWhiteCandidatesHaveDistinctMovingWhiteAndLitSandFloor(t *testing.T) {
	keys := visual.Layout()
	for _, variant := range []string{"sand-pearl", "sand-ripple"} {
		maxStep, whiteSeen, maxSpread := 0, false, 0
		for _, key := range keys {
			previous := candidateColor(variant, key, 0)
			for n := 1; n <= 600; n++ {
				c := candidateColor(variant, key, float64(n)/10)
				if c[0] < 26 || c[1] < 16 || c[2] < 7 || c[0] > 72 || c[1] > 72 || c[2] > 72 {
					t.Fatal("floor/ceiling", variant, c)
				}
				if c[0] > 60 && c[2] > 55 {
					whiteSeen = true
				}
				for ch, v := range c {
					maxStep = max(maxStep, int(math.Abs(float64(v)-float64(previous[ch]))))
				}
				previous = c
			}
		}
		for n := 0; n < 200; n++ {
			lo, hi := 255, 0
			for _, k := range keys {
				c := candidateColor(variant, k, float64(n)/10)
				lo = min(lo, int(c[2]))
				hi = max(hi, int(c[2]))
			}
			maxSpread = max(maxSpread, hi-lo)
		}
		if !whiteSeen || maxSpread < 40 || maxStep > 12 {
			t.Fatalf("%s no visible smooth white: white=%v spread=%d step=%d", variant, whiteSeen, maxSpread, maxStep)
		}
		t.Logf("%s: max 100ms channel step=%d, max spatial blue contrast=%d", variant, maxStep, maxSpread)
	}
}
