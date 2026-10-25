package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"github.com/parsiya/f108-pro/pkg/aula"
	"math"
	"strconv"
	"testing"
	"time"
)

func TestAllKeysUseCurrentEffectsIncludingEveryFunctionKey(t *testing.T) {
	for _, state := range []status.Kind{status.Thinking, status.Tool, status.Editing, status.Approval, status.Coding, status.Responding, status.Failed, status.Complete} {
		r := New()
		r.Start = time.Unix(0, 0)
		tasks := []status.Task{{Thread: "main", State: state, Updated: r.Start}}
		lows, highs := map[uint8]uint8{}, map[uint8]uint8{}
		for n := 1; n <= 12; n++ {
			lows[aula.KeyNameToIndex["f"+strconv.Itoa(n)]] = 255
		}
		for n := 0; n < 120; n++ {
			frame := r.Render(tasks, r.Start.Add(time.Duration(n)*100*time.Millisecond))
			if len(frame) != 104 {
				t.Fatal("LED map incomplete")
			}
			for idx, c := range frame {
				for _, v := range c {
					if v > 165 {
						t.Fatal("65% cap exceeded")
					}
				}
				if n < 6 {
					continue
				}
				if state == status.Coding {
					if c[0] != c[1] || c[2] < c[0] {
						t.Fatal("coding white/blue hue changed", c)
					}
				} else if state != status.Thinking && !(state == status.Complete && n > 20) {
					h := colorFor(state, DefaultPalette)
					for ch, v := range c {
						if h[ch] == 0 && v != 0 {
							t.Fatal("hue changed")
						}
					}
				} else if state == status.Thinking && (c[0] != c[1] || c[2] < c[0]) {
					t.Fatal("thinking gained cyan")
				}
				if _, ok := lows[idx]; ok {
					peak := c[0]
					if c[1] > peak {
						peak = c[1]
					}
					if c[2] > peak {
						peak = c[2]
					}
					if peak < lows[idx] {
						lows[idx] = peak
					}
					if peak > highs[idx] {
						highs[idx] = peak
					}
				}
			}
		}
		for idx, lo := range lows {
			if int(highs[idx])-int(lo) < 8 {
				t.Fatalf("F row not animated for %s LED%d: %d..%d", state, idx, lo, highs[idx])
			}
		}
	}
}
func TestSecondaryTasksCannotOverrideFRowOrRestartAnimation(t *testing.T) {
	a, b := New(), New()
	start := time.Unix(0, 0)
	a.Start = start
	b.Start = start
	one := []status.Task{{Thread: "main", State: status.Tool, Updated: start}}
	many := append([]status.Task{}, one...)
	for n := 0; n < 15; n++ {
		many = append(many, status.Task{Thread: strconv.Itoa(n), State: status.Failed, Updated: start})
	}
	for n := 0; n < 30; n++ {
		at := start.Add(time.Duration(n) * 100 * time.Millisecond)
		x, y := a.Render(one, at), b.Render(many, at)
		for idx, c := range x {
			if y[idx] != c {
				t.Fatalf("secondary task changed LED%d", idx)
			}
		}
	}
}
func TestConnectionContinuityAndTimeBasedAnimation(t *testing.T) {
	start := time.Unix(0, 0)
	a, b := New(), New()
	a.Start = start
	b.Start = start
	tasks := []status.Task{{Thread: "t", State: status.Tool, Updated: start}}
	for _, r := range []*Renderer{&a, &b} {
		r.Render(tasks, start)
		r.Render(tasks, start.Add(time.Second))
	}
	tasks[0].State = status.Editing
	for _, r := range []*Renderer{&a, &b} {
		r.Render(tasks, start.Add(2*time.Second))
	}
	for n := 1; n < 5; n++ {
		a.Render(tasks, start.Add(2*time.Second+time.Duration(n)*time.Second/10))
	}
	for n := 1; n < 8; n++ {
		b.Render(tasks, start.Add(2*time.Second+time.Duration(n)*time.Second/15))
	}
	x, y := a.Render(tasks, start.Add(2500*time.Millisecond)), b.Render(tasks, start.Add(2500*time.Millisecond))
	for idx, c := range x {
		if y[idx] != c {
			t.Fatal("FPS changed animation time")
		}
	}
	tasks[0].State = status.Failed
	next := a.Render(tasks, start.Add(2500*time.Millisecond))
	for idx, c := range x {
		if next[idx] != c {
			t.Fatal("interrupted transition jumped")
		}
	}
}
func TestDocumentDirectionAndWholeKeyboardApproval(t *testing.T) {
	peakAt := func(y float64) float64 {
		best, at := 0., 0.
		for n := 0; n < 54; n++ {
			s := float64(n) / 15
			v := documentFlow(6, y, s)
			if v > best {
				best, at = v, s
			}
		}
		return at
	}
	if peakAt(5)-peakAt(2) < .6 {
		t.Fatal("document direction changed")
	}
	r := New()
	r.Start = time.Unix(0, 0)
	tasks := []status.Task{{Thread: "t", State: status.Approval, Updated: r.Start}}
	for n := 0; n < 10; n++ {
		r.Render(tasks, r.Start.Add(time.Duration(n)*100*time.Millisecond))
	}
	for _, name := range []string{"f1", "f12", "space", "num5", "right", "enter"} {
		k := keyGeometry[aula.KeyNameToIndex[name]]
		low, high := 1., 0.
		for n := 0; n < 80; n++ {
			v := approvalCurtain(k.X+k.W/2, float64(n)/10)
			low = math.Min(low, v)
			high = math.Max(high, v)
		}
		if high-low < .3 {
			t.Fatal("approval not visible", name)
		}
	}
}
func BenchmarkCurrentRenderer(b *testing.B) {
	r := New()
	tasks := []status.Task{{Thread: "t", State: status.Thinking, Updated: r.Start}}
	b.ReportAllocs()
	for n := 0; n < b.N; n++ {
		r.Render(tasks, r.Start.Add(time.Duration(n)*100*time.Millisecond))
	}
}
