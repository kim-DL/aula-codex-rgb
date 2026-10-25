package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"math"
	"testing"
	"time"
)

func TestRefinedMotionRemainsReadableAt10FPS(t *testing.T) {
	for _, kind := range []status.Kind{status.Tool, status.Thinking} {
		r := New()
		r.Start = time.Unix(0, 0)
		tasks := []status.Task{{Thread: "t", State: kind, Tool: "exec", ActiveTools: 1, Updated: r.Start}}
		var previous map[uint8]RGB
		maxStep := 0
		for n := 0; n < 1200; n++ {
			frame := r.Render(tasks, r.Start.Add(time.Duration(n)*100*time.Millisecond))
			for idx, c := range frame {
				if n > 10 {
					for ch, v := range c {
						step := int(v) - int(previous[idx][ch])
						if step < 0 {
							step = -step
						}
						maxStep = max(maxStep, step)
					}
				}
				if kind == status.Thinking && (c[0] != c[1] || c[0] > c[2]) && n > 10 {
					t.Fatal("cyan/new thinking hue")
				}
			}
			previous = frame
		}
		t.Logf("%s maximum per-key 100ms channel step: %d/255", kind, maxStep)
		if maxStep > 95 {
			t.Fatalf("%s moves too abruptly at 10fps: %d", kind, maxStep)
		}
	}
}

func TestToolVariationPreservesShuttleAndUsesOnlyIdentifiableMetadata(t *testing.T) {
	for _, x := range []float64{2, 8, 16, 21} {
		tm := 1.0
		head := 11.5 + 10.5*math.Sin(tm*2*math.Pi/4)
		if math.Abs(toolMotion(x, 2, tm, "exec", 1)-(.15+.85*gaussian(head-x, 1.5))) > 1e-9 {
			t.Fatal("familiar shuttle changed")
		}
	}
	different := 0
	for n := 0; n < 120; n++ {
		s := float64(n) / 10
		for _, x := range []float64{3, 9, 15, 21} {
			a := toolMotion(x, 2, s, "exec", 1)
			if a != toolMotion(x, 2, s, "unknown", 1) {
				t.Fatal("unknown tool guessed a category")
			}
			if math.Abs(a-toolMotion(x, 2, s, "read_file", 1)) > .1 {
				different++
			}
		}
	}
	if different < 20 {
		t.Fatal("identifiable reading tool has no visible variation")
	}
}

func TestNewMotionUsesTimeInsteadOfFrameCount(t *testing.T) {
	for _, kind := range []status.Kind{status.Thinking, status.Tool} {
		a, b := New(), New()
		a.Start = time.Unix(0, 0)
		b.Start = a.Start
		task := []status.Task{{Thread: "t", State: kind, Updated: a.Start}}
		for n := 0; n <= 100; n++ {
			a.Render(task, a.Start.Add(time.Duration(n)*100*time.Millisecond))
		}
		for n := 0; n <= 150; n++ {
			b.Render(task, b.Start.Add(time.Duration(n)*time.Second/15))
		}
		x, y := a.Render(task, a.Start.Add(10*time.Second)), b.Render(task, b.Start.Add(10*time.Second))
		for idx, c := range x {
			if y[idx] != c {
				t.Fatal("frame rate changed the motion")
			}
		}
	}
}
