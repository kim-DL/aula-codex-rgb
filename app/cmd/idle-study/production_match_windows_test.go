//go:build windows

package main

import (
	"aula-codex-rgb-lab/internal/visual"
	"math"
	"testing"
	"time"
)

// Preserve the independently exercised 65% prototype when porting it to the
// globally scaled resident renderer. Quantization can differ by one code value.
func TestResidentIdleMatchesChosenHardwarePrototype(t *testing.T) {
	r := visual.New()
	r.Start = time.Unix(0, 0)
	r.Render(nil, r.Start)
	maxDifference := 0.
	for n := 20; n <= 900; n++ {
		sec := float64(n) / 10
		f := r.Render(nil, r.Start.Add(time.Duration(n)*100*time.Millisecond))
		for _, key := range visual.Layout() {
			want := refinedColor("sand-pearl-refined", key, sec)
			got := f[key.Index]
			for ch, v := range got {
				maxDifference = math.Max(maxDifference, math.Abs(float64(v)-float64(want[ch])))
			}
		}
	}
	if maxDifference > 1 {
		t.Fatal("resident does not match chosen prototype", maxDifference)
	}
	t.Log("maximum prototype/production quantization difference:", maxDifference)
}
