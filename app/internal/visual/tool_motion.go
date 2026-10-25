package visual

import (
	"math"
	"strings"
)

// A twelve-second choreography, independent of frame count or task progress.
// Keep the familiar shuttle for six seconds, then cross and stagger the beams.
func toolMotion(x, y, t float64, tool string, active int) float64 {
	head := 11.5 + 10.5*math.Sin(t*2*math.Pi/4)
	classic := .15 + .85*gaussian(head-x, 1.5)
	other := 11.5 + 10.2*math.Sin(t*2*math.Pi/5.2)
	crossing := .12 + .88*math.Max(gaussian(x-other-y*.23, 1.45), .78*gaussian(x-(23-other)+y*.23, 1.9))
	lane := 11.5 + 10.5*math.Sin(t*2*math.Pi/4.8+y*.39)
	staggered := .13 + .87*gaussian(x-lane, 1.65)
	phase := wrap(t, 12)
	name := strings.ToLower(tool)
	// Names are evidence; do not infer commands hidden inside generic wrappers.
	if strings.Contains(name, "search") || strings.Contains(name, "read") || strings.Contains(name, "fetch") {
		phase = wrap(phase+6, 12)
	}
	var level float64
	switch {
	case phase < 6:
		level = classic + (crossing-classic)*smoothUnit((phase-5.1)/.9)
	case phase < 9:
		level = crossing + (staggered-crossing)*smoothUnit((phase-8.1)/.9)
	default:
		level = staggered + (classic-staggered)*smoothUnit((phase-11.1)/.9)
	}
	if active > 1 {
		// Additional real executions add a faint counterflow, without a new hue.
		level = math.Max(level, .13+.42*gaussian(x-(23-head), 2.1))
	}
	return math.Min(1, level)
}
