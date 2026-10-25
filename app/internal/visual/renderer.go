package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"github.com/parsiya/f108-pro/pkg/aula"
	"math"
	"time"
)

type RGB = [3]uint8
type Palette struct{ Idle, Thinking, Responding, Coding, Editing, Tool, Approval, Complete, Failed RGB }

var DefaultPalette = Palette{
	Idle: RGB{44, 28, 12}, Thinking: RGB{0, 0, 255}, Responding: RGB{255, 255, 255},
	Coding: RGB{0, 0, 255}, Editing: RGB{255, 255, 255}, Tool: RGB{255, 255, 0},
	Approval: RGB{255, 255, 0}, Complete: RGB{0, 255, 0}, Failed: RGB{255, 0, 0},
}
var rows = [][]string{
	{"esc", "f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12", "printscreen", "scrolllock", "pause"},
	{"grave", "1", "2", "3", "4", "5", "6", "7", "8", "9", "0", "minus", "equal", "backspace", "insert", "home", "pageup", "numlock", "numslash", "numstar", "numminus"},
	{"tab", "q", "w", "e", "r", "t", "y", "u", "i", "o", "p", "lbracket", "rbracket", "backslash", "delete", "end", "pagedown", "num7", "num8", "num9", "numplus"},
	{"capslock", "a", "s", "d", "f", "g", "h", "j", "k", "l", "semicolon", "quote", "enter", "num4", "num5", "num6"},
	{"lshift", "z", "x", "c", "v", "b", "n", "m", "comma", "dot", "slash", "rshift", "up", "num1", "num2", "num3", "numenter"},
	{"lctrl", "lwin", "lalt", "space", "ralt", "fn", "menu", "rctrl", "left", "down", "right", "num0", "numdot"},
}

type Renderer struct {
	Brightness         uint8
	IdleStatic         bool // Pause keeps a steady dim rest frame.
	Start              time.Time
	stateTimes         map[string]stateMoment
	generation         uint64
	previous, bodyFrom map[uint8]RGB
	bodyAt             time.Time
	bodyMode           connectionMode
	lastMain           status.Task
	initialized        bool
}

func New() Renderer { return Renderer{Brightness: 65, Start: time.Now()} }
func scale(c RGB, b uint8) RGB {
	var out RGB
	for i := range c {
		out[i] = uint8(uint16(c[i]) * uint16(b) / 100)
	}
	return out
}
func mix(a, b RGB, x float64) RGB {
	x = math.Max(0, math.Min(1, x))
	var out RGB
	for i := range out {
		out[i] = uint8(math.Round(float64(a[i])*(1-x) + float64(b[i])*x))
	}
	return out
}
func (r *Renderer) Render(tasks []status.Task, now time.Time) aula.LEDFrame {
	if now.IsZero() {
		now = time.Now()
	}
	r.trackStateTimes(tasks, now)
	frame := make(aula.LEDFrame, len(keyGeometry))
	for idx := range keyGeometry {
		var c RGB
		if len(tasks) == 0 {
			c = r.calmIdleColor(idx, now)
		} else {
			if tasks[0].State == status.Thinking {
				c = r.livingThinkingColor(tasks[0], idx, now)
			} else {
				c = r.detailColor(tasks[0], idx, now)
			}
		}
		frame[idx] = scale(c, r.Brightness)
	}
	if r.previous == nil {
		r.previous = make(map[uint8]RGB, len(frame))
		for idx := range frame {
			r.previous[idx] = scale(DefaultPalette.Idle, r.Brightness)
		}
	}
	r.connectBody(frame, tasks, now)
	for idx, c := range frame {
		r.previous[idx] = c
	}
	return frame
}
func colorFor(k status.Kind, p Palette) RGB {
	switch k {
	case status.Thinking:
		return p.Thinking
	case status.Responding:
		return p.Responding
	case status.Coding:
		return p.Coding
	case status.Editing, status.Reading:
		return p.Editing
	case status.Tool:
		return p.Tool
	case status.Approval:
		return p.Approval
	case status.Complete:
		return p.Complete
	case status.Failed, status.Interrupted:
		return p.Failed
	default:
		return p.Idle
	}
}
