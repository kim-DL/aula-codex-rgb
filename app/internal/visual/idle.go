package visual

import (
	"math"
	"time"
)

// The only default idle effect: warm sand with a restrained moving white glow.
// This returns unscaled colors; Renderer.Render applies the user's global cap.
func (r *Renderer) calmIdleColor(idx uint8, now time.Time) RGB {
	sand, white := RGB{40, 25, 11}, RGB{111, 111, 111} // ~26/16/7 .. 72/72/72 at 65%.
	if r.IdleStatic {
		return sand
	}
	t := math.Max(0, now.Sub(r.Start).Seconds())
	k := keyGeometry[idx]
	x, y := k.X+k.W/2, k.Y+k.H/2
	center := func(sec float64) (float64, float64) {
		phase := 2*math.Pi*sec/15.6 + .15*math.Sin(2*math.Pi*sec/37)
		return 11.7 - 9.5*math.Cos(phase), 3.1 + .60*math.Sin(2*math.Pi*sec/23)
	}
	cx, cy := center(t)
	nx, ny := center(t - 1.05)
	fx, fy := center(t - 2.5)
	soft := func(px, py, sx, sy float64) float64 {
		return math.Exp(-.5 * (math.Pow((x-px)/sx, 2) + math.Pow((y-py)/sy, 2)))
	}
	core := soft(cx, cy, 2.45, 2.85)
	near := soft(nx, ny, 3.10, 3.25)
	far := soft(fx, fy, 4.25, 3.75)
	lift := .985 + .015*math.Sin(2*math.Pi*t/9.7)
	alpha := (.94*core + .25*near*(1-core) + .075*far*(1-core)*(1-near)) * lift
	return mix(sand, white, alpha)
}
