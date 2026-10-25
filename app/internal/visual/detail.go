package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"github.com/parsiya/f108-pro/pkg/aula"
	"math"
	"sort"
	"time"
)

type stateMoment struct {
	State status.Kind
	Turn  string
	At    time.Time
	Seen  uint64
}

func (r *Renderer) trackStateTimes(tasks []status.Task, now time.Time) {
	if r.stateTimes == nil {
		r.stateTimes = map[string]stateMoment{}
	}
	r.generation++
	for _, task := range tasks {
		old, ok := r.stateTimes[task.Thread]
		if !ok || old.State != task.State || old.Turn != task.Turn {
			at := task.Updated
			if at.IsZero() {
				at = now
			}
			old = stateMoment{State: task.State, Turn: task.Turn, At: at}
		}
		old.Seen = r.generation
		r.stateTimes[task.Thread] = old
	}
	for thread, m := range r.stateTimes {
		if m.Seen != r.generation {
			delete(r.stateTimes, thread)
		}
	}
}

type LayoutKey struct {
	Name       string `json:"name"`
	Index      uint8  `json:"index"`
	X, Y, W, H float64
}

var keyGeometry = buildGeometry()

func Layout() []LayoutKey {
	out := make([]LayoutKey, 0, len(keyGeometry))
	for _, k := range keyGeometry {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Y != out[j].Y {
			return out[i].Y < out[j].Y
		}
		return out[i].X < out[j].X
	})
	return out
}
func buildGeometry() map[uint8]LayoutKey {
	m := map[uint8]LayoutKey{}
	add := func(name string, x, y, w, h float64) {
		if idx, ok := aula.KeyNameToIndex[name]; ok {
			m[idx] = LayoutKey{name, idx, x, y, w, h}
		}
	}
	for row, names := range rows {
		x := 0.0
		y := float64(row) + .5
		if row == 0 {
			y = 0
		}
		for _, name := range names {
			w, h := 1.0, 1.0
			switch name {
			case "tab", "backslash":
				w = 1.5
			case "capslock":
				w = 1.75
			case "lshift", "enter":
				w = 2.25
			case "rshift":
				w = 2.75
			case "backspace", "num0":
				w = 2
			case "space":
				w = 6.25
			case "lctrl", "lwin", "lalt", "ralt", "fn", "menu", "rctrl":
				w = 1.25
			}
			if name == "numplus" || name == "numenter" {
				h = 2
			}
			switch name {
			case "f1":
				x = 2
			case "f5":
				x = 6.5
			case "f9":
				x = 11
			case "printscreen", "insert", "delete", "left":
				x = 15.5
			case "up":
				x = 16.5
			case "numlock", "num7", "num4", "num1", "num0":
				x = 19
			}
			add(name, x, y, w, h)
			x += w
		}
	}
	return m
}
func gaussian(d, width float64) float64 { return math.Exp(-d * d / (2 * width * width)) }
func wrap(v, period float64) float64    { return v - period*math.Floor(v/period) }
func intensity(c RGB, v float64) RGB {
	v = math.Max(0, math.Min(1, v))
	var out RGB
	for ch := range out {
		out[ch] = uint8(math.Round(float64(c[ch]) * v))
	}
	return out
}

func (r *Renderer) detailColor(task status.Task, idx uint8, now time.Time) RGB {
	k := keyGeometry[idx]
	x, y := k.X+k.W/2, k.Y+k.H/2
	t := now.Sub(r.Start).Seconds()
	age := math.Max(0, now.Sub(r.stateTimes[task.Thread].At).Seconds())
	hue := colorFor(task.State, DefaultPalette)
	if task.State == status.Editing || task.State == status.Reading {
		return intensity(hue, documentFlow(x, y, age))
	}
	if task.State == status.Approval {
		return intensity(hue, approvalCurtain(x, age))
	}
	if task.State == status.Coding {
		if k.Y == 0 && k.Name != "esc" && k.Name != "printscreen" && k.Name != "scrolllock" && k.Name != "pause" {
			// F keys share the letter-row scanner; no independent status badges.
			y, _, _ = codingScanPosition(age)
		}
		return codingScanColor(x, y, age)
	}
	level := .16
	d := math.Hypot((x-7.5)*.65, (y-3.5)*1.3)
	switch task.State {
	case status.Responding:
		head := wrap(t*3.2+y*.8, 17) - 1
		behind := head - x
		level = .17 + .8*gaussian(behind, 1.8)
		if behind > 0 {
			level += .18 * math.Exp(-behind/5)
		}
	case status.Tool:
		level = toolMotion(x, y, t, task.Tool, task.ActiveTools)
	case status.Complete:
		ring := math.Abs(d - age*3.2)
		level = 100.0/255 + .60*gaussian(ring, 1.15)*math.Exp(-age/5)
	case status.Failed, status.Interrupted:
		level = .4
		if age < 2.8 {
			pulse := math.Sin(age * math.Pi / 1.4)
			level = .4 + .55*pulse*pulse
		}
		if idx == aula.KeyEsc {
			level = .95
		}
	default:
		return r.calmIdleColor(idx, now)
	}
	level = .22 + (level-.13)*.82
	color := intensity(hue, level)
	if task.State == status.Complete && age > 2 {
		// Keep the established green completion wave, then settle into the
		// same continuously timed idle field before the eight-second cue expires.
		color = mix(color, r.calmIdleColor(idx, now), smoothUnit((age-2)/4))
	}
	return color
}
