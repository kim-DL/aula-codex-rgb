//go:build windows

package main

import (
	"aula-codex-rgb-lab/internal/visual"
	"math"
	"testing"
)

func TestRefinedWhiteMotionKeepsAcceptedVisibilityWithGentlerChanges(t *testing.T) {
	keys := visual.Layout()
	for _, v := range []string{"sand-pearl-refined", "sand-ripple-refined"} {
		maxStep, maxBlueSpread, whitest, maxLitFraction := 0, 0, 0, 0.
		for _, key := range keys {
			previous := refinedColor(v, key, 0)
			peakBlue := 0
			for n := 1; n <= 900; n++ {
				c := refinedColor(v, key, float64(n)/10)
				if c[0] < 26 || c[1] < 16 || c[2] < 7 || c[0] > 72 || c[1] > 72 || c[2] > 72 {
					t.Fatal("floor or peak changed", c)
				}
				for ch, b := range c {
					maxStep = max(maxStep, int(math.Abs(float64(b)-float64(previous[ch]))))
				}
				peakBlue = max(peakBlue, int(c[2]))
				previous = c
			}
			if peakBlue < 18 {
				t.Fatalf("key %s stopped participating in %s", key.Name, v)
			}
			whitest = max(whitest, peakBlue)
		}
		for n := 0; n < 900; n++ {
			lo, hi, lit := 255, 0, 0
			for _, key := range keys {
				c := refinedColor(v, key, float64(n)/10)
				lo = min(lo, int(c[2]))
				hi = max(hi, int(c[2]))
				if c[2] >= 40 {
					lit++
				}
			}
			maxBlueSpread = max(maxBlueSpread, hi-lo)
			maxLitFraction = math.Max(maxLitFraction, float64(lit)/float64(len(keys)))
		}
		if whitest < 63 || maxBlueSpread < 50 || maxStep > 8 || maxLitFraction > .65 {
			t.Fatalf("%s lost intent: peak=%d spread=%d step=%d coverage=%g", v, whitest, maxBlueSpread, maxStep, maxLitFraction)
		}
		t.Logf("%s: max 100ms step=%d (previous 9), blue peak=%d, blue spatial spread=%d, bright-key coverage max %.1f%%", v, maxStep, whitest, maxBlueSpread, maxLitFraction*100)
	}
}
func TestRefinedRippleCycleHasNoSeamFlash(t *testing.T) {
	for _, key := range visual.Layout() {
		before := refinedColor("sand-ripple-refined", key, 10.3999)
		after := refinedColor("sand-ripple-refined", key, 10.4001)
		for ch, b := range before {
			if math.Abs(float64(b)-float64(after[ch])) > 1 {
				t.Fatal("ripple seam flash", key.Name)
			}
		}
	}
}
