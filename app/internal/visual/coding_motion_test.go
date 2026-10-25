package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"math"
	"testing"
	"time"
)

func TestCodingWhiteHeadBlueFieldAndOneKeyAfterglow(t *testing.T) {
	row, head, _ := codingScanPosition(1.6)
	center := codingScanColor(head, row, 1.6)
	background := codingScanColor(22.5, 5, 1.6)
	tail := codingScanColor(head-.95, row, 1.6)
	farTail := codingScanColor(head-2.5, row, 1.6)
	ahead := codingScanColor(head+.95, row, 1.6)
	if center[0] < 230 || center[0] != center[1] || center[1] != center[2] {
		t.Fatal("white center is tinted", center)
	}
	if background[0] != 0 || background[1] != 0 || background[2] < 65 {
		t.Fatal("blue background lost", background)
	}
	if tail[0] < 80 || tail[1] < 60 || tail[0] >= center[0] || tail[0] <= ahead[0] {
		t.Fatal("directional tail missing", tail, ahead)
	}
	if farTail[0] > 3 || farTail[1] > 3 {
		t.Fatal("long colored afterglow remains", farTail)
	}
}
func TestCodingBoundariesFadeAndNeverFlashOtherRows(t *testing.T) {
	for _, boundary := range []float64{4, 8, 12, 16} {
		for _, key := range keyGeometry {
			x, y := key.X+key.W/2, key.Y+key.H/2
			a, b := codingScanColor(x, y, boundary-.001), codingScanColor(x, y, boundary+.001)
			for channel := range a {
				if math.Abs(float64(a[channel])-float64(b[channel])) > 2 {
					t.Fatal("hard row reset", boundary, key.Name, a, b)
				}
			}
		}
	}
}

func TestWhiteScanKeepsCoreStrengthAndBackground(t *testing.T) {
	row, head, _ := codingScanPosition(1.6)
	// Keep the restrained core strength from the preceding gold refinement.
	mid := codingScanColor(head+.4, row, 1.6)
	lift := float64(mid[0]) / 210
	if lift < 1.05 || lift > 1.08 || mid[0] != mid[1] || mid[1] != mid[2] {
		t.Fatal("core strength/neutrality changed", mid, lift)
	}
	if background := codingScanColor(22.5, 5, 1.6); background != (RGB{0, 0, 74}) {
		t.Fatal("blue background changed", background)
	}
}
func TestCoding65PercentCapAndEveryFunctionKeySharesScanner(t *testing.T) {
	r := New()
	at := time.Unix(100, 0)
	r.Start = at
	whiteKeys := map[string]bool{}
	backgroundSeen := false
	for n := 0; n < 300; n++ {
		frame := r.Render([]status.Task{{Thread: "code", State: status.Coding, Updated: at}}, at.Add(time.Duration(n)*100*time.Millisecond))
		for idx, c := range frame {
			if c[0] > 165 || c[1] > 165 || c[2] > 165 {
				t.Fatal("brightness cap exceeded")
			}
			if n > 6 && (c[0] != c[1] || c[2] < c[0]) {
				t.Fatal("white accent gained a colored tint", c)
			}
			key := keyGeometry[idx]
			if c[0] > 110 && c[0] == c[1] && key.Y == 0 {
				whiteKeys[key.Name] = true
			}
			if c[2] > 40 && c[0] == 0 {
				backgroundSeen = true
			}
		}
	}
	for _, name := range []string{"f1", "f2", "f3", "f4", "f5", "f6", "f7", "f8", "f9", "f10", "f11", "f12"} {
		if !whiteKeys[name] {
			t.Fatal("F key excluded", name)
		}
	}
	if !backgroundSeen {
		t.Fatal("blue field missing")
	}
}
