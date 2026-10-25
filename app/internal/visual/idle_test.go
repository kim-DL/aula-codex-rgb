package visual

import (
	"maps"
	"testing"
	"time"

	"aula-codex-rgb-lab/internal/status"
	"github.com/parsiya/f108-pro/pkg/aula"
)

func TestSandWhiteIdleVisibleContinuousAndAllFunctionKeys(t *testing.T) {
	r := New()
	r.Start = time.Unix(0, 0)
	low, high := map[uint8]uint8{}, map[uint8]uint8{}
	var previous aula.LEDFrame
	for n := 0; n <= 400; n++ {
		frame := r.Render(nil, r.Start.Add(time.Duration(n)*100*time.Millisecond))
		for idx, c := range frame {
			if c[0] > 72 || c[1] > 72 || c[2] > 72 || c[0] < 26 || c[1] < 16 || c[2] < 7 || c[0] < c[1] || c[1] < c[2] {
				t.Fatalf("idle cap/hue/blackout: %v", c)
			}
			if previous != nil {
				for ch, v := range c {
					d := int(v) - int(previous[idx][ch])
					if d < -7 || d > 7 {
						t.Fatal("idle channel jumped", d)
					}
				}
			}
			if n == 0 || c[0] < low[idx] {
				low[idx] = c[0]
			}
			if c[0] > high[idx] {
				high[idx] = c[0]
			}
		}
		previous = frame
	}
	for _, k := range Layout() {
		if int(high[k.Index])-int(low[k.Index]) < 12 {
			t.Fatal("key did not join subtle motion", k.Name)
		}
	}
}

func TestCompletionKeepsGreenThenSettlesWithoutIdleJump(t *testing.T) {
	r := New()
	r.Start = time.Unix(0, 0)
	task := []status.Task{{Thread: "t", Turn: "1", State: status.Complete, Updated: r.Start}}
	r.Render(task, r.Start)
	green := r.Render(task, r.Start.Add(time.Second))
	for _, c := range green {
		if c[0] != 0 || c[2] != 0 || c[1] < 50 {
			t.Fatal("completion cue lost", c)
		}
	}
	settled := r.Render(task, r.Start.Add(6*time.Second))
	for idx, c := range settled {
		if c != scale(r.calmIdleColor(idx, r.Start.Add(6*time.Second)), 65) {
			t.Fatal("completion did not settle", c)
		}
	}
	before := r.Render(task, r.Start.Add(8*time.Second))
	after := r.Render(nil, r.Start.Add(8*time.Second))
	if !maps.Equal(before, after) {
		t.Fatal("completion expired with a jump")
	}
}

func TestPausedIdleSteadyAndWorkImmediatelyResumes(t *testing.T) {
	r := New()
	r.Start = time.Unix(0, 0)
	r.IdleStatic = true
	r.Render(nil, r.Start)
	a := r.Render(nil, r.Start.Add(3*time.Second))
	b := r.Render(nil, r.Start.Add(20*time.Second))
	if !maps.Equal(a, b) {
		t.Fatal("paused lighting moved")
	}
	r.IdleStatic = false
	task := []status.Task{{Thread: "t", State: status.Tool, Updated: r.Start.Add(20 * time.Second)}}
	r.Render(task, r.Start.Add(20*time.Second))
	work := r.Render(task, r.Start.Add(21*time.Second))
	for _, c := range work {
		if c[0] != c[1] || c[2] != 0 {
			t.Fatal("tool hue changed", c)
		}
	}
}
func TestIdleRespectsGlobalBrightnessAndPauseHasNoWhiteHighlight(t *testing.T) {
	for _, brightness := range []uint8{0, 25, 65, 100} {
		r := New()
		r.Start = time.Unix(0, 0)
		r.Brightness = brightness
		r.Render(nil, r.Start)
		for n := 20; n < 300; n++ {
			f := r.Render(nil, r.Start.Add(time.Duration(n)*100*time.Millisecond))
			for _, c := range f {
				for _, v := range c {
					if int(v) > 111*int(brightness)/100 {
						t.Fatal("global cap ignored", brightness, c)
					}
				}
			}
		}
	}
	r := New()
	r.Start = time.Unix(0, 0)
	r.IdleStatic = true
	r.Render(nil, r.Start)
	a := r.Render(nil, r.Start.Add(3*time.Second))
	for _, c := range a {
		if c != (RGB{26, 16, 7}) {
			t.Fatal("pause did not preserve static sand", c)
		}
	}
}
