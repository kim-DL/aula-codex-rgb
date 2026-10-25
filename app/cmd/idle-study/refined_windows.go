//go:build windows

package main

import (
	"aula-codex-rgb-lab/internal/visual"
	"math"
)

// Development-only design iteration; the resident idle renderer is unchanged.
func refinedColor(variant string, k visual.LayoutKey, t float64) [3]uint8 {
	t = math.Max(0, t)
	x, y := k.X+k.W/2, k.Y+k.H/2
	var alpha float64
	if variant == "sand-pearl-refined" {
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
		alpha = (.94*core + .25*near*(1-core) + .075*far*(1-core)*(1-near)) * lift
	} else {
		period := 10.4
		phase := math.Mod(t, period)
		cx := 10.3 + .65*math.Sin(2*math.Pi*t/31)
		cy := 3.0 + .30*math.Sin(2*math.Pi*t/27)
		dx, dy := x-cx, (y-cy)*1.42
		distance := math.Hypot(dx, dy)
		// A slight curvature variation avoids a ruler-straight scan without noise.
		distance += .16 * math.Sin(math.Atan2(dy, dx)*2+2*math.Pi*t/29)
		radius := phase*2.85 - 5
		front := math.Exp(-.5 * math.Pow((distance-radius)/2.05, 2))
		wake := math.Exp(-.5 * math.Pow((distance-(radius-2.6))/2.7, 2))
		after := math.Exp(-.5 * math.Pow((distance-(radius-5.0))/3.1, 2))
		envelope := math.Pow(math.Sin(math.Pi*phase/period), 2)
		alpha = (.94*front + .20*wake*(1-front) + .055*after*(1-front)*(1-wake)) * envelope
	}
	alpha = math.Max(0, math.Min(1, alpha))
	sand, white := [3]uint8{26, 16, 7}, [3]uint8{72, 72, 72}
	var out [3]uint8
	for ch := range out {
		out[ch] = uint8(math.Round(float64(sand[ch]) + (float64(white[ch])-float64(sand[ch]))*alpha))
	}
	return out
}
