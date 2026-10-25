package visual

import (
	"math"
	"testing"
)

func TestThinkingExtraSpotUsesTheThreeLetterRows(t *testing.T) {
	for _, cy := range []float64{2.9, 3.1, 3.49} {
		if thinkingExtensionTarget(cy, -.15) != 4 || thinkingExtensionTarget(cy, .15) != 4 {
			t.Fatal("top row must point below")
		}
	}
	for _, cy := range []float64{4.51, 4.9, 5.1} {
		if thinkingExtensionTarget(cy, -.15) != 4 || thinkingExtensionTarget(cy, .15) != 4 {
			t.Fatal("bottom row must point above")
		}
	}
	if thinkingExtensionTarget(4, .15) < 4.95 || thinkingExtensionTarget(4, -.15) > 3.05 {
		t.Fatal("middle must select one neighbor according to travel")
	}
}
func TestRowAwareThinkingHasNoExtensionOutsideLetterRowsOrSuddenTargetJump(t *testing.T) {
	keys := Layout()
	extraSum, baseSum, maxAdded := 0, 0, 0
	maxTargetStep := 0.
	visited := map[string]bool{}
	previousTarget := 4.
	for n := 0; n < 1200; n++ {
		sec := float64(n) / 10
		cx, cy, v := thinkingWhitePosition(sec)
		target := thinkingExtensionTarget(cy, v)
		if target < 3 || target > 5 {
			t.Fatal("extension escaped letter rows", target)
		}
		if n > 0 {
			maxTargetStep = math.Max(maxTargetStep, math.Abs(target-previousTarget))
		}
		previousTarget = target
		if cy < 3.5 {
			visited["top"] = true
		} else if cy > 4.5 {
			visited["bottom"] = true
		} else {
			visited["middle"] = true
		}
		breath := math.Pow(.5+.5*math.Sin(sec*2*math.Pi/6.3+.24*math.Sin(sec*.19)), 1.6)
		width := .66 + .18*breath
		baseCount, extraCount := 0, 0
		for _, k := range keys {
			x, y := k.X+k.W/2, k.Y+k.H/2
			base := math.Exp(-.5 * (math.Pow((x-cx)*.52/width, 2) + math.Pow((y-cy)*1.45/width, 2)))
			got := thinkingWhiteCore(x, y, sec, breath)
			if base >= .5 {
				baseCount++
			}
			if got >= .5 {
				extraCount++
			}
			if (y < 2.75 || y > 5.25) && math.Abs(got-base) > 1e-12 {
				t.Fatal("extra light outside letter rows", k.Name)
			}
			if math.Abs(x-cx) > 1.2 && got >= .5 && base < .5 {
				t.Fatal("extra light became an entire row")
			}
		}
		baseSum += baseCount
		extraSum += extraCount
		maxAdded = max(maxAdded, extraCount-baseCount)
	}
	added := float64(extraSum-baseSum) / 1200
	t.Logf("base %.2f keys, with extra %.2f, average added %.2f, max added/frame %d, max target move/100ms %.3f rows, visited=%v", float64(baseSum)/1200, float64(extraSum)/1200, added, maxAdded, maxTargetStep, visited)
	if added < .40 || added > 1.40 || maxAdded > 2 || maxTargetStep > .12 || len(visited) != 3 {
		t.Fatal("row policy lost smoothness or small-spot scope")
	}
}
