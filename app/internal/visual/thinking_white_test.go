package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"
)

func TestCompactThinkingKeepsOriginalBlueAndRestrainsWhite(t *testing.T) {
	b, err := os.ReadFile("testdata/thinking_before_20261025.json")
	if err != nil {
		t.Fatal(err)
	}
	var baseline struct {
		Keys   []LayoutKey
		Frames [][]uint32
	}
	if err = json.Unmarshal(b, &baseline); err != nil {
		t.Fatal(err)
	}
	r := New()
	r.Start = time.Unix(0, 0)
	tasks := []status.Task{{Thread: "preview", State: status.Thinking, Updated: r.Start}}
	var oldEnergy, newEnergy uint64
	oldMax, newMax := 0, 0
	for n, colors := range baseline.Frames {
		frame := r.Render(tasks, r.Start.Add(time.Duration(n)*100*time.Millisecond))
		if n < 20 {
			continue
		}
		for i, k := range baseline.Keys {
			old := RGB{uint8(colors[i] >> 16), uint8(colors[i] >> 8), uint8(colors[i])}
			got := frame[k.Index]
			if got[2] != old[2] || got[0] != got[1] || got[0] > got[2] {
				t.Fatal("blue, neutral hue or existing highlight changed", k.Name, old, got)
			}
			oldEnergy += uint64(old[0])
			newEnergy += uint64(got[0])
			oldMax = max(oldMax, int(old[0]))
			newMax = max(newMax, int(got[0]))
		}
	}
	ratio := float64(newEnergy) / float64(oldEnergy)
	t.Logf("neutral signal sum %.2fx (large prototype was5.53x), original/new peak %d/%d", ratio, oldMax, newMax)
	if ratio < 1.1 || ratio > 1.80 || newMax > 165 {
		t.Fatal("lost original blue/white balance")
	}
}
func TestCompactThinkingRemainsSmoothAtTenFPS(t *testing.T) {
	r := New()
	r.Start = time.Unix(0, 0)
	tasks := []status.Task{{Thread: "t", State: status.Thinking, Updated: r.Start}}
	var previous map[uint8]RGB
	maxStep := 0
	for n := 0; n < 1200; n++ {
		f := r.Render(tasks, r.Start.Add(time.Duration(n)*100*time.Millisecond))
		if n > 20 {
			for idx, c := range f {
				for ch, v := range c {
					maxStep = max(maxStep, int(math.Abs(float64(v)-float64(previous[idx][ch]))))
				}
			}
		}
		previous = f
	}
	t.Logf("maximum 100ms per-key step=%d/255", maxStep)
	if maxStep > 40 {
		t.Fatal("compact pulse became abrupt")
	}
}
