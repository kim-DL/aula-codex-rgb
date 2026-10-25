package visual

import (
	"math"
	"time"

	"aula-codex-rgb-lab/internal/status"
)

// An artistic breathing/heartbeat metaphor, not a measurement of the model.
// Every component uses elapsed seconds, never frame count or fresh random noise.
// Equal red/green channels keep blue-to-white highlights free of cyan accents.
func (r *Renderer) livingThinkingColor(task status.Task, idx uint8, now time.Time) RGB {
	k := keyGeometry[idx]
	x, y := k.X+k.W/2, k.Y+k.H/2
	t := math.Max(0, now.Sub(r.Start).Seconds())
	age := math.Max(0, now.Sub(r.stateTimes[task.Thread].At).Seconds())
	cx := 8.1 + 1.7*math.Sin(t*.21) + .55*math.Sin(t*.49+.3)
	cy := 3.3 + .53*math.Sin(t*.25+.8)
	d := math.Hypot((x-cx)*.74, (y-cy)*1.45)
	d2 := math.Hypot((x-(14.5+1.2*math.Sin(t*.16+1.7)))*.8, (y-(2.6+.65*math.Sin(t*.19)))*1.35)
	breathPhase := t*2*math.Pi/6.3 + .24*math.Sin(t*.19)
	breath := math.Pow(.5+.5*math.Sin(breathPhase), 1.6)
	// Preserve the legacy outward wave while adding a slow, moving origin.
	wave := math.Pow(.5+.5*math.Sin(t*1.95-d*.70+.25*math.Sin(t*.31+y*.3)), 2.1)
	blue := .012 + .67*wave*(.48+.52*breath)
	blue += .18 * gaussian(d, 1.1+breath*.9)
	// Two soft pulses per cycle, delayed through the surrounding key field.
	beatPhase := t/3.2 + .04*math.Sin(t*.17) - d*.055
	phase := wrap(beatPhase, 1)
	pulse := periodicPulse(phase, .20, .082) + .45*periodicPulse(phase, .44, .105)
	core := math.Max(gaussian(d, .66+.18*breath), (.24+.25*breath)*gaussian(d2, .82))
	blue += .34 * core * (.45 + .55*pulse)
	blue += .10 * math.Exp(-age/1.8) * gaussian(d-age*1.6, .85)
	blue = math.Max(0, math.Min(1, blue))
	// Keep the established blue wave, pulse timing and row-aware white mask.
	// A slightly deeper release makes the accepted compact nucleus breathe
	// more visibly. Its footprint, period and secondary accent stay unchanged.
	primaryWhite := .97 * thinkingWhiteCore(x, y, t, breath) * (.30 + .70*pulse)
	secondaryWhite := .94 * (.24 + .25*breath) * gaussian(d2, .82) * (.35 + .65*pulse)
	white := math.Min(1, math.Max(primaryWhite, secondaryWhite))
	channel := uint8(math.Round(255 * blue))
	highlight := uint8(math.Round(float64(channel) * white))
	// Lift only the existing neutral channels by 8%, bounded by the original
	// blue channel. Zero remains zero; geometry, timing and blue are untouched.
	highlight = uint8(math.Min(float64(channel), math.Round(float64(highlight)*1.08)))
	return RGB{highlight, highlight, channel}
}

func periodicPulse(phase, center, width float64) float64 {
	return gaussian(phase-center, width) + gaussian(phase-center-1, width) + gaussian(phase-center+1, width)
}

// Keep the accepted horizontal width. Only the neutral white nucleus visits
// the three letter rows; the original blue wave is calculated above unchanged.
func thinkingWhitePosition(t float64) (cx, cy, velocity float64) {
	cx = 8.1 + 1.7*math.Sin(t*.21) + .55*math.Sin(t*.49+.3)
	cy = 4 - .92*math.Cos(t*.18)
	velocity = .92 * .18 * math.Sin(t*.18)
	return
}

// The extra spot points into the letter field at its top and bottom. In the
// middle it follows the travel direction. Blending through the main nucleus
// avoids a discontinuous jump or two independently bright extension spots.
func thinkingExtensionTarget(cy, velocity float64) float64 {
	middle := smoothUnit((cy-3.5)/.35) * (1 - smoothUnit((cy-4.15)/.35))
	return 4 + middle*math.Tanh(velocity/.035)
}
func thinkingWhiteCore(x, y, t, breath float64) float64 {
	cx, cy, velocity := thinkingWhitePosition(t)
	width := .66 + .18*breath
	horizontal := math.Exp(-.5 * (math.Pow((x-cx)*.52/width, 2) + math.Pow((y-cy)*1.45/width, 2)))
	if y < 2.75 || y > 5.25 {
		return horizontal
	}
	targetY := thinkingExtensionTarget(cy, velocity)
	extra := math.Exp(-.5 * (math.Pow((x-cx)/.60, 2) + math.Pow((y-targetY)/.50, 2)))
	return math.Max(horizontal, .97*extra)
}
