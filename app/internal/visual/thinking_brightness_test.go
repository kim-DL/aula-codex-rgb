package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"
)

func TestThinkingWhiteBrightnessOnlyPreservesBlueTimingAndLitKeys(t *testing.T) {
	b, err := os.ReadFile("testdata/thinking_row_before_brightness_20261025.json")
	if err != nil {
		t.Fatal(err)
	}
	var old struct {
		Keys   []LayoutKey
		Frames [][]uint32
	}
	if err = json.Unmarshal(b, &old); err != nil {
		t.Fatal(err)
	}
	r := New()
	r.Start = time.Unix(0, 0)
	tasks := []status.Task{{Thread: "preview", State: status.Thinking, Updated: r.Start}}
	var beforeSum, afterSum uint64
	changed := 0
	beforeMax, afterMax := 0, 0
	for n, packed := range old.Frames {
		frame := r.Render(tasks, r.Start.Add(time.Duration(n)*100*time.Millisecond))
		if n < 20 {
			continue
		}
		for i, k := range old.Keys {
			previous := RGB{uint8(packed[i] >> 16), uint8(packed[i] >> 8), uint8(packed[i])}
			now := frame[k.Index]
			if now[2] != previous[2] {
				t.Fatal("blue value changed", n, k.Name, previous, now)
			}
			if now[0] != now[1] || now[0] < previous[0] || now[0] > now[2] {
				t.Fatal("white lift lost neutrality or bounds", n, k.Name)
			}
			if (now[0] == 0) != (previous[0] == 0) {
				t.Fatal("new highlight key appeared or disappeared", n, k.Name)
			}
			upper := int(math.Ceil(float64(previous[0])*1.08)) + 2
			if int(now[0]) > upper {
				t.Fatal("lift exceeded 8% with rounding", n, k.Name, previous, now)
			}
			beforeSum += uint64(previous[0])
			afterSum += uint64(now[0])
			beforeMax = max(beforeMax, int(previous[0]))
			afterMax = max(afterMax, int(now[0]))
			if now[0] > previous[0] {
				changed++
			}
		}
	}
	ratio := float64(afterSum) / float64(beforeSum)
	t.Logf("40-second neutral channel sum %.4fx, peak %d->%d, brighter key-frames=%d, blue and zero-support identical", ratio, beforeMax, afterMax, changed)
	if ratio < 1.04 || ratio > 1.10 || changed < 100 {
		t.Fatal("white-only brightness lift was not restrained/effective")
	}
}
