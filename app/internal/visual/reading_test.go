package visual

import (
	"aula-codex-rgb-lab/internal/status"
	"maps"
	"testing"
	"time"
)

func TestReadFramesExactlyReuseDocumentEffectAndTransition(t *testing.T) {
	a, b := New(), New()
	at := time.Unix(100, 0)
	a.Start = at
	b.Start = at
	for n := 0; n < 140; n++ {
		stateA, stateB := status.Tool, status.Tool
		if n >= 10 {
			stateA, stateB = status.Editing, status.Reading
		}
		now := at.Add(time.Duration(n) * 100 * time.Millisecond)
		left := a.Render([]status.Task{{Thread: "t", Turn: "1", State: stateA, Updated: at.Add(time.Second)}}, now)
		right := b.Render([]status.Task{{Thread: "t", Turn: "1", State: stateB, Updated: at.Add(time.Second)}}, now)
		if !maps.Equal(left, right) {
			t.Fatalf("read effect differs at frame %d", n)
		}
	}
}
