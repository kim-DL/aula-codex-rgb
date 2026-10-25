// Package lcdtrial is a bounded, RAM-only clock experiment. It contains no I/O
// or persistence and retains exactly one completed duration.
package lcdtrial

import "time"

type Trial struct {
	phase    int
	phaseAt  time.Time
	lastSync time.Time
	recent   time.Duration
}

func New(now time.Time) *Trial { return &Trial{phaseAt: now} }

func phaseLength(p int) time.Duration {
	switch p {
	case 0:
		return 10 * time.Second
	case 3:
		return 8 * time.Second
	case 1:
		return 12 * time.Second
	case 2, 4:
		return 15 * time.Second
	case 5:
		return 6 * time.Second
	}
	return 0
}

func (t *Trial) Phase() string {
	return [...]string{"pc_clock", "count_first", "hold_recent", "count_new", "hold_new_recent", "restore_pc_clock", "done"}[t.phase]
}
func (t *Trial) Recent() time.Duration { return t.recent }

// Next returns a calendar value only when a serialized clock transaction is
// needed. No prior completed durations or per-second samples are retained.
func (t *Trial) Next(now time.Time, reconnect bool) (clock time.Time, send bool, done bool) {
	if t.phase == 6 {
		return time.Time{}, false, true
	}
	if now.Sub(t.phaseAt) >= phaseLength(t.phase) {
		if t.phase == 1 || t.phase == 3 {
			t.recent = now.Sub(t.phaseAt).Truncate(time.Second)
		}
		t.phase++
		t.phaseAt = now
		t.lastSync = time.Time{}
		if t.phase == 6 {
			return time.Time{}, false, true
		}
	}
	holding := t.phase == 2 || t.phase == 4
	send = reconnect || t.lastSync.IsZero() || holding && now.Sub(t.lastSync) >= 200*time.Millisecond
	if !send {
		return time.Time{}, false, false
	}
	t.lastSync = now
	if t.phase == 0 || t.phase == 5 {
		return now, true, false
	}
	elapsed := now.Sub(t.phaseAt)
	if holding {
		elapsed = t.recent
	}
	s := int(elapsed/time.Second) % (24 * 3600)
	return time.Date(now.Year(), now.Month(), now.Day(), s/3600, s/60%60, s%60, 0, now.Location()), true, false
}
