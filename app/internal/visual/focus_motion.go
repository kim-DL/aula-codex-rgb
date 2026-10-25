package visual

import "math"

func smoothUnit(v float64) float64 {
	v = math.Max(0, math.Min(1, v))
	return v * v * (3 - 2*v)
}

// One clear downward focal path, with a short trailing sheet and a settling
// glow. A visual metaphor for editing, not file-save percentage/progress.
func documentFlow(x, y, age float64) float64 {
	phase := wrap(age, 4.8)
	level := .14
	if phase < 3.6 {
		q := phase / 3.6
		head := -1 + 10*q
		distance := head - (y + x*.075)
		envelope := smoothUnit(q/.10) * smoothUnit((1-q)/.16)
		level += envelope * (.80*gaussian(distance, .52) + .24*gaussian(distance-1.1, .95))
	} else {
		settle := math.Sin(math.Pi * (phase - 3.6) / 1.2)
		level += .40 * settle * settle
	}
	return math.Min(1, level)
}

// Full-height opposing amber curtains converge and reopen over 5.6 seconds.
// No key-specific overrides: navigation, arrows and numpad move with the body.
// max rather than addition keeps the meeting point from flashing brighter.
func approvalCurtain(x, age float64) float64 {
	phase := age * 2 * math.Pi / 5.6
	openness := .5 + .5*math.Cos(phase)
	left, right := 11.6-10.6*openness, 11.6+10.6*openness
	band := math.Max(gaussian(x-left, 2.1), gaussian(x-right, 2.1))
	return .13 + .16*(1-openness) + .69*band
}
