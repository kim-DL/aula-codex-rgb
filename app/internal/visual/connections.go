package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"github.com/parsiya/f108-pro/pkg/aula"
	"math"
	"time"
)

type connectionMode string

const (
	fade     connectionMode = "fade"
	gather   connectionMode = "gather"
	downward connectionMode = "downward"
	opening  connectionMode = "opening"
	arrival  connectionMode = "arrival"
	urgent   connectionMode = "urgent"
)

func connectionFor(from, to status.Kind) connectionMode {
	if to == status.Failed || to == status.Interrupted {
		return urgent
	}
	if to == status.Complete {
		return arrival
	}
	if from == status.Approval {
		return opening
	}
	if from == status.Thinking && to == status.Tool {
		return gather
	}
	if from == status.Tool && (to == status.Editing || to == status.Reading) {
		return downward
	}
	return fade
}
func connectionDuration(mode connectionMode) float64 {
	if mode == urgent {
		return .20
	}
	if mode == fade {
		return .35
	}
	return .55
}

// One transition path for the whole keyboard, including every F key.
func (r *Renderer) connectBody(displayed aula.LEDFrame, tasks []status.Task, now time.Time) {
	main := status.Task{State: status.Idle}
	if len(tasks) > 0 {
		main = tasks[0]
	}
	changed := !r.initialized || main.Thread != r.lastMain.Thread || main.Turn != r.lastMain.Turn || main.State != r.lastMain.State
	if changed {
		r.bodyMode = fade
		if r.initialized && r.lastMain.Thread != "" && r.lastMain.Thread == main.Thread && r.lastMain.Turn == main.Turn {
			r.bodyMode = connectionFor(r.lastMain.State, main.State)
		}
		r.bodyFrom = make(map[uint8]RGB, len(displayed))
		for idx := range displayed {
			r.bodyFrom[idx] = r.previous[idx]
		}
		r.bodyAt, r.lastMain, r.initialized = now, main, true
	}
	duration := connectionDuration(r.bodyMode)
	if main.State == status.Idle {
		duration = 1.8
	}
	p := math.Max(0, now.Sub(r.bodyAt).Seconds()/duration)
	for idx, target := range displayed {
		if p >= 1 {
			displayed[idx] = target
			continue
		}
		k := keyGeometry[idx]
		x, y := k.X+k.W/2, k.Y+k.H/2
		position := 0.0
		switch r.bodyMode {
		case gather:
			position = x / 23.5
		case downward:
			position = (y/6.5 + .12*x/23.5) / 1.12
		case opening:
			position = math.Abs(x-11.75) / 11.75
		case arrival:
			position = math.Min(1, math.Hypot((x-9)/15, (y-3)/6))
		}
		alpha := p
		if r.bodyMode != fade && r.bodyMode != urgent {
			alpha = smoothUnit((p - .38*math.Min(1, position)) / .62)
		}
		if r.bodyMode == urgent {
			alpha = smoothUnit(p)
		}
		from := r.bodyFrom[idx]
		if r.bodyMode == gather {
			// Brief contraction of the displayed blue wave, no new accent color.
			contraction := 1 - .42*p*(1-gaussian(x-(9+5*p), 5-3*p))
			for ch := range from {
				from[ch] = uint8(math.Round(float64(from[ch]) * contraction))
			}
		}
		displayed[idx] = mix(from, target, alpha)
	}
}
