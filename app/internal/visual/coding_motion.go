package visual

import "math"

// The three letter rows have centers y=3,4,5 in the verified wired key map.
// Position depends only on elapsed time, never the number of sent frames.
func codingScanPosition(age float64) (row, head, envelope float64) {
	age = math.Max(0, age)
	phase := wrap(age, 4)
	row = 3 + float64(int(age/4)%3)
	head = 1.8 + 12.6*smoothUnit(phase/3.2)
	envelope = smoothUnit(phase/.28) * (1 - smoothUnit((phase-3.2)/.8))
	return
}

// A steady blue field and a compact neutral-white cursor with a one-key glow.
// Equal red/green channels avoid a cyan or magenta accent. The blue floor
// remains steady; the brighter cursor replaces it with neutral white.
// This is activity motion, not file-save or actual line-number progress.
func codingScanColor(x, y, age float64) RGB {
	row, head, envelope := codingScanPosition(age)
	distance := head - x
	vertical := gaussian(y-row, .46)
	core := gaussian(distance, .68) * vertical
	tail := 0.0
	if distance > 0 && distance < 1.65 {
		tail = .28 * gaussian(distance-.9, .34) * smoothUnit(distance/.35) * (1 - smoothUnit((distance-1.2)/.45)) * gaussian(y-row, .46)
	}
	// A restrained lift only in the cursor core. Keep the one-key tail and
	// low-level support unchanged; the renderer still enforces the 65% cap.
	coreGain := 1 + .06*smoothUnit((core-.20)/.45)
	strength := math.Min(1, (.98*core*coreGain+tail)*envelope)
	white := uint8(math.Round(255 * strength))
	blue := uint8(math.Round(255 * math.Max(.29, strength)))
	return RGB{white, white, blue}
}
