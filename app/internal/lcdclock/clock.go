// Package lcdclock owns one volatile LCD timer. It has no persistence or USB I/O.
package lcdclock

import (
	"strings"
	"sync"
	"time"
)

type Event struct {
	Thread, Turn, Type, Source string
	At, ReceivedAt             time.Time
}

type Plan struct {
	Revision uint64
	Mode     string
	Time     time.Time
}

// Status is available only by a live, same-user pipe query. Never add this
// structure to disk health reports, diagnostics, settings, or event history.
type Status struct {
	Enabled        bool   `json:"desktop_running"`
	Active         bool   `json:"active"`
	Thread         string `json:"thread,omitempty"`
	Turn           string `json:"turn,omitempty"`
	Source         string `json:"source,omitempty"`
	ElapsedSeconds int64  `json:"elapsed_seconds"`
	HasRecent      bool   `json:"has_recent"`
	RecentSeconds  int64  `json:"recent_seconds"`
	DesiredMode    string `json:"desired_mode"`
	WrittenMode    string `json:"written_mode"`
	Writes         uint64 `json:"clock_writes"`
	Errors         uint64 `json:"clock_errors"`
	Suspended      bool   `json:"suspended"`
}

type Clock struct {
	mu                                        sync.Mutex
	enabled, active, hasRecent                bool
	thread, turn, source                      string
	anchor, newestStart                       time.Time
	recent                                    time.Duration
	revision, writtenRevision, writes, errors uint64
	writtenMode                               string
	lastWrite, retryAt                        time.Time
	failures                                  int
}

func New() *Clock { return &Clock{revision: 1} }

func (c *Clock) SetEnabled(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.enabled == enabled {
		return
	}
	c.enabled = enabled
	if !enabled {
		c.active = false
	}
	c.revision++
}

// Accept is called only after the RGB engine has accepted the event. Session
// discovery, tool activity, Stop hints, and subagents cannot start this timer.
func (c *Clock) Accept(e Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.enabled || strings.Contains(e.Thread, "#subagent:") {
		return
	}
	switch e.Type {
	case "UserPromptSubmit", "turn/started":
		if e.At.IsZero() || e.At.After(e.ReceivedAt.Add(2*time.Second)) || e.ReceivedAt.Sub(e.At) >= 24*time.Hour {
			return
		}
		if c.active && e.Thread == c.thread && (e.Turn == c.turn || e.Turn == "" || c.turn == "") {
			if c.active && c.turn == "" {
				c.turn = e.Turn
			}
			return
		}
		if !c.active && e.Thread == c.thread && e.Turn != "" && e.Turn == c.turn {
			return
		}
		if !c.newestStart.IsZero() && !e.At.After(c.newestStart) {
			return
		}
		c.thread, c.turn, c.source = e.Thread, e.Turn, e.Source
		// Keep a monotonic anchor even when the authoritative start came from JSON.
		c.anchor = e.ReceivedAt.Add(-max(time.Duration(0), e.ReceivedAt.Sub(e.At)))
		c.newestStart = e.At
		c.active = true
		c.revision++
	case "task_complete", "turn/completed", "Interrupt", "SessionEnd":
		if !c.active || e.Thread != c.thread || (c.turn != "" && e.Turn != "" && e.Turn != c.turn) {
			return
		}
		if e.At.Before(c.newestStart) {
			return
		}
		c.recent = max(time.Duration(0), e.At.Sub(c.newestStart))
		c.hasRecent = true
		c.active = false
		c.revision++
	}
}

func (c *Clock) mode(now time.Time) string {
	if c.enabled && c.active && now.Sub(c.anchor) < 24*time.Hour {
		return "elapsed"
	}
	return "pc_clock"
}

// Next requests boundary/reconnect updates, never per-second rewrites. The
// keyboard RTC advances on its own. The date always remains the PC's date.
func (c *Clock) Next(now time.Time, force bool) (Plan, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failures >= 3 || now.Before(c.retryAt) {
		return Plan{}, false
	}
	mode := c.mode(now)
	dateChanged := !c.lastWrite.IsZero() && (now.Year() != c.lastWrite.Year() || now.YearDay() != c.lastWrite.YearDay())
	jump := !c.lastWrite.IsZero() && abs(time.Duration(now.UnixNano()-c.lastWrite.UnixNano())-now.Sub(c.lastWrite)) > 2*time.Second
	if !force && c.revision == c.writtenRevision && mode == c.writtenMode && !dateChanged && !jump && now.Sub(c.lastWrite) < 15*time.Minute {
		return Plan{}, false
	}
	t := now
	if mode == "elapsed" {
		seconds := int64(max(time.Duration(0), now.Sub(c.anchor)) / time.Second)
		t = time.Date(now.Year(), now.Month(), now.Day(), int(seconds/3600), int(seconds/60%60), int(seconds%60), 0, now.Location())
	}
	return Plan{c.revision, mode, t}, true
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func (c *Clock) Commit(p Plan, now time.Time, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		c.errors++
		c.failures++
		c.retryAt = now.Add(time.Duration(c.failures) * 5 * time.Second)
		return
	}
	c.failures = 0
	c.retryAt = time.Time{}
	c.writes++
	c.writtenRevision = p.Revision
	c.writtenMode = p.Mode
	c.lastWrite = now
}

func (c *Clock) Status(now time.Time) Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := Status{Enabled: c.enabled, Active: c.active, Thread: c.thread, Turn: c.turn, Source: c.source, HasRecent: c.hasRecent, RecentSeconds: int64(c.recent / time.Second), DesiredMode: c.mode(now), WrittenMode: c.writtenMode, Writes: c.writes, Errors: c.errors, Suspended: c.failures >= 3}
	if c.active {
		s.ElapsedSeconds = int64(max(time.Duration(0), now.Sub(c.anchor)) / time.Second)
	}
	return s
}
