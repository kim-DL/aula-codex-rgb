package status

import (
	"aula-codex-rgb-lab/internal/lcdclock"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Kind string

const changeAfterglow = 1800 * time.Millisecond
const responseAfterglow = 1600 * time.Millisecond

func fileCue(k Kind) bool { return k == Coding || k == Editing || k == Reading }

const (
	Idle        Kind = "idle"
	Thinking    Kind = "thinking"
	Responding  Kind = "responding"
	Coding      Kind = "coding"
	Editing     Kind = "editing"
	Reading     Kind = "reading"
	Tool        Kind = "tool"
	Approval    Kind = "approval"
	Complete    Kind = "complete"
	Failed      Kind = "failed"
	Interrupted Kind = "interrupted"
)

type Event struct {
	Activity                                         Kind
	ActivityConfirmed                                bool
	ID, Thread, Turn, Call, Type, Tool, Path, Source string
	Error                                            bool
	Temporary                                        bool
	At, ReceivedAt                                   time.Time
	Confirmed                                        bool
}
type Task struct {
	Thread, Turn                  string
	State                         Kind
	Tool                          string
	Since, Updated                time.Time
	Confirmed                     bool
	DoneUntil, ReceivedAt         time.Time
	Source, Event                 string
	HoldUntil, ResumeAt           time.Time
	RecoverableError              bool
	ActiveTools, PendingApprovals int
	Stale                         bool
}
type callState struct {
	Tool      string
	At        time.Time
	Activity  Kind
	Confirmed bool
}
type lifecycle struct {
	started, terminal, touched time.Time
	turn                       string
	tools, waits               map[string]callState
	ended                      map[string]time.Time
}
type Engine struct {
	clock            *lcdclock.Clock
	mu               sync.RWMutex
	tasks            map[string]Task
	seen             map[string]time.Time
	lives            map[string]*lifecycle
	temporaryIgnored uint64
}

func New() *Engine {
	return &Engine{clock: lcdclock.New(), tasks: map[string]Task{}, seen: map[string]time.Time{}, lives: map[string]*lifecycle{}}
}

func (e *Engine) Clock() *lcdclock.Clock { return e.clock }

// A retained rollout start can recover the elapsed time after a resident
// restart or a hook arriving after its tool. It must match an already active,
// identified live turn; it never creates an RGB task or replays old completion.
func (e *Engine) RecoverClockStart(v Event) bool {
	if v.Temporary || v.Source != "rollout" || v.Type != "UserPromptSubmit" || v.Turn == "" {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	l := e.lives[v.Thread]
	if l == nil || !l.terminal.IsZero() || l.turn != v.Turn {
		return false
	}
	if _, ok := e.tasks[v.Thread]; !ok {
		return false
	}
	e.clockEvent(v)
	return true
}

func (e *Engine) clockEvent(v Event) {
	e.clock.Accept(lcdclock.Event{Thread: v.Thread, Turn: v.Turn, Type: v.Type, Source: v.Source, At: v.At, ReceivedAt: v.ReceivedAt})
}

func (e *Engine) IgnoredTemporaryEvents() uint64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.temporaryIgnored
}
func newLife(v Event) *lifecycle {
	return &lifecycle{started: v.At, touched: v.ReceivedAt, turn: v.Turn, tools: map[string]callState{}, waits: map[string]callState{}, ended: map[string]time.Time{}}
}
func isStart(s string) bool {
	return s == "UserPromptSubmit" || s == "SessionStart" || s == "turn/started" || s == "SubagentStart"
}
func known(s string) bool {
	switch s {
	case "UserPromptSubmit", "SessionStart", "turn/started", "SubagentStart", "PreToolUse", "item/started", "PermissionRequest", "PostToolUse", "item/completed", "AgentMessage", "item/agentMessage/delta", "FileChange", "item/fileChange/delta", "task_complete", "turn/completed", "Interrupt", "Stop", "SessionEnd", "SubagentStop":
		return true
	}
	return false
}
func fileKind(path string) Kind {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != "" && strings.Contains(" .go .rs .js .jsx .ts .tsx .py .java .c .h .cpp .cs .swift .kt .rb .php .vue .svelte .html .css .json .yaml .yml .toml .sql .ps1 .sh ", " "+ext+" ") {
		return Coding
	}
	return Editing
}
func callKey(v Event) string {
	if v.Call != "" {
		return v.Call
	}
	// Name-only events cannot be matched to an identified concurrent execution.
	return "anonymous:" + v.Tool
}
func running(t *Task, l *lifecycle) {
	t.ActiveTools, t.PendingApprovals = len(l.tools), len(l.waits)
	if len(l.waits) > 0 {
		t.State, t.Confirmed = Approval, true
		return
	}
	if len(l.tools) > 0 {
		t.State, t.Confirmed = Tool, true
		keys := make([]string, 0, len(l.tools))
		for key := range l.tools {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		t.Tool = l.tools[keys[0]].Tool
		for _, key := range keys {
			call := l.tools[key]
			if call.Activity == Reading {
				t.State, t.Confirmed, t.Tool = Reading, call.Confirmed, call.Tool
				break
			}
		}
		return
	}
	t.State, t.Confirmed, t.Tool = Thinking, false, ""
}
func (e *Engine) Apply(v Event) bool {
	if v.Temporary {
		e.mu.Lock()
		e.temporaryIgnored++
		e.mu.Unlock()
		return false
	}
	if v.Thread == "" || !known(v.Type) {
		return false
	}
	if v.At.IsZero() {
		v.At = time.Now()
	}
	if v.ReceivedAt.IsZero() {
		v.ReceivedAt = time.Now()
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	t := e.tasks[v.Thread]
	l := e.lives[v.Thread]
	if v.ID != "" {
		if _, seen := e.seen[v.ID]; seen {
			// A call may have a pre phase both before permission and on actual
			// execution. Only a newer pre for that outstanding wait can resume it.
			resume := false
			if l != nil && v.Type == "PreToolUse" {
				wait, pending := l.waits[callKey(v)]
				resume = pending && v.At.After(wait.At)
			}
			if !resume {
				// A metadata-only hook can arrive before its richer rollout start.
				// Enrich the SAME outstanding call without adding/restarting a tool.
				if v.Type == "PreToolUse" && v.Call != "" && v.Activity == Reading && l != nil && l.terminal.IsZero() && (v.Turn == "" || l.turn == "" || v.Turn == l.turn) {
					if call, active := l.tools[callKey(v)]; active && call.Activity != Reading {
						call.Activity, call.Confirmed = Reading, v.ActivityConfirmed
						l.tools[callKey(v)] = call
						if t.State == Tool || t.State == Thinking {
							running(&t, l)
							t.Since = v.ReceivedAt
							t.ReceivedAt = v.ReceivedAt
							t.Source = v.Source
							e.tasks[v.Thread] = t
						}
						return true
					}
				}
				return false
			}
		}
	}
	if l == nil {
		l = newLife(v)
		e.lives[v.Thread] = l
	}
	if isStart(v.Type) && (!l.terminal.IsZero() || (v.Turn != "" && l.turn != "" && v.Turn != l.turn)) {
		if !v.At.After(t.Updated) || (!l.terminal.IsZero() && !v.At.After(l.terminal)) {
			return false
		}
		if v.Turn != "" && v.Turn == l.turn {
			return false
		}
		l = newLife(v)
		e.lives[v.Thread] = l
		t = Task{}
	}
	if !l.terminal.IsZero() && v.Type != "SessionEnd" && v.Type != "SubagentStop" {
		return false
	}
	if v.Turn != "" && l.turn != "" && v.Turn != l.turn {
		return false
	}
	if v.At.Before(l.started) {
		return false
	}
	// Hooks without timestamps use delivery time. A persisted terminal record
	// may therefore be older than a late hook for the SAME identified turn.
	// Never apply that exception to an unidentified or different-turn terminal.
	matchedTerminal := v.Turn != "" && v.Turn == l.turn && (v.Type == "task_complete" || v.Type == "turn/completed" || v.Type == "Interrupt")
	if v.At.Before(t.Updated) && !matchedTerminal && (v.Call == "" || (v.Type != "PostToolUse" && v.Type != "PreToolUse" && v.Type != "PermissionRequest")) {
		return false
	}
	key := callKey(v)
	if v.Call != "" && (v.Type == "PreToolUse" || v.Type == "PermissionRequest") {
		if _, ended := l.ended[key]; ended {
			return false
		}
	}
	if t.Thread == "" {
		t = Task{Thread: v.Thread, Since: v.At}
	}
	if v.Turn != "" {
		t.Turn = v.Turn
		l.turn = v.Turn
	}
	if v.Type != "Stop" && (v.At.After(t.Updated) || t.Updated.IsZero()) {
		t.Updated = v.At
	}
	t.ReceivedAt, t.Source, t.Event, t.Stale = v.ReceivedAt, v.Source, v.Type, false
	l.touched = v.ReceivedAt
	previous := t.State
	previousConfirmed := t.Confirmed
	finished, hadCall := l.tools[key]
	reading := v.Activity == Reading || (hadCall && finished.Activity == Reading)
	readingConfirmed := v.ActivityConfirmed
	if v.Activity == "" && hadCall {
		readingConfirmed = finished.Confirmed
	}
	switch v.Type {
	case "UserPromptSubmit", "SessionStart", "turn/started", "SubagentStart":
		// Duplicate start sources must not clear executions already seen.
		running(&t, l)
	case "PreToolUse", "item/started":
		if v.Call == "" {
			delete(l.ended, key)
		}
		l.tools[key] = callState{Tool: v.Tool, At: v.At, Activity: v.Activity, Confirmed: v.ActivityConfirmed}
		delete(l.waits, key)
		t.ResumeAt = time.Time{}
		running(&t, l)
		if reading && previous == Failed && t.RecoverableError && v.ReceivedAt.Before(t.DoneUntil) {
			t.State, t.Confirmed = Failed, true
		}
		if fileCue(previous) && v.ReceivedAt.Before(t.HoldUntil) {
			// Keep a confirmed file-change cue visible while tracking new tools.
			t.State, t.ResumeAt = previous, t.HoldUntil
			t.Confirmed = previousConfirmed
		}
	case "PermissionRequest":
		l.waits[key] = callState{Tool: v.Tool, At: v.At}
		t.ResumeAt = time.Time{}
		running(&t, l)
	case "PostToolUse", "item/completed":
		delete(l.tools, key)
		delete(l.waits, key)
		l.ended[key] = v.ReceivedAt
		// An unidentified outer wrapper cannot release a distinct identified wait.
		if len(l.waits) > 0 {
			running(&t, l)
		} else if v.Error {
			t.State, t.Confirmed, t.RecoverableError = Failed, true, true
			t.DoneUntil = v.ReceivedAt.Add(4 * time.Second)
		} else if reading {
			if previous == Failed && t.RecoverableError && v.ReceivedAt.Before(t.DoneUntil) {
				// Preserve the existing error cue over a successful read.
			} else if (previous == Coding || previous == Editing) && v.ReceivedAt.Before(t.HoldUntil) {
				t.ResumeAt = t.HoldUntil
			} else {
				t.State, t.Confirmed = Reading, readingConfirmed
				t.ResumeAt = v.ReceivedAt.Add(changeAfterglow)
			}
		} else if v.Path != "" {
			t.State, t.Confirmed = fileKind(v.Path), true
			t.ResumeAt = v.ReceivedAt.Add(changeAfterglow)
		} else if strings.HasSuffix(v.Tool, "apply_patch") || v.Tool == "Edit" || v.Tool == "Write" {
			t.State, t.Confirmed = Editing, true
			if previous == Coding || previous == Editing {
				t.State = previous
			}
			t.ResumeAt = v.ReceivedAt.Add(changeAfterglow)
		} else if previous == Failed && t.RecoverableError {
			// Successful wrappers do not hide the four-second error cue.
		} else if !t.HoldUntil.IsZero() && v.ReceivedAt.Before(t.HoldUntil) && (previous == Tool || fileCue(previous) || previous == Responding) {
			t.ResumeAt = t.HoldUntil
		} else {
			running(&t, l)
			t.ResumeAt = time.Time{}
		}
	case "AgentMessage", "item/agentMessage/delta":
		if len(l.waits) > 0 {
			running(&t, l)
		} else if fileCue(previous) && v.ReceivedAt.Before(t.HoldUntil) {
			t.State, t.ResumeAt = previous, t.HoldUntil
		} else if len(l.tools) > 0 {
			running(&t, l)
		} else {
			t.State, t.Confirmed, t.Tool = Responding, true, ""
			t.ResumeAt = v.ReceivedAt.Add(responseAfterglow)
		}
	case "FileChange", "item/fileChange/delta":
		if len(l.waits) > 0 {
			running(&t, l)
		} else {
			t.State, t.Confirmed = fileKind(v.Path), true
			t.ResumeAt = v.ReceivedAt.Add(changeAfterglow)
		}
		if v.Error && len(l.waits) == 0 {
			t.State, t.Confirmed, t.RecoverableError = Failed, true, true
			t.DoneUntil = v.ReceivedAt.Add(4 * time.Second)
			t.ResumeAt = time.Time{}
		}
	case "task_complete", "turn/completed", "Interrupt":
		l.terminal = v.At
		// The authoritative boundary also governs subsequent turn starts;
		// synthetic timestamps from old hooks must not block a real new turn.
		t.Updated = v.At
		l.tools = map[string]callState{}
		l.waits = map[string]callState{}
		t.ResumeAt = time.Time{}
		t.Tool = ""
		t.Confirmed = true
		t.RecoverableError = false
		t.State = Complete
		t.DoneUntil = v.ReceivedAt.Add(8 * time.Second)
		if v.Error {
			t.State = Failed
			t.DoneUntil = v.ReceivedAt.Add(4 * time.Second)
		}
		if v.Type == "Interrupt" {
			t.State = Interrupted
			t.DoneUntil = v.ReceivedAt.Add(4 * time.Second)
		}
	case "Stop":
		// Hint only, never an authoritative completion.
	case "SessionEnd", "SubagentStop":
		l.terminal = v.At
		l.tools = map[string]callState{}
		l.waits = map[string]callState{}
		if t.State != Complete {
			delete(e.tasks, v.Thread)
			e.clockEvent(v)
			return true
		}
	}
	if t.State != previous {
		t.Since = v.ReceivedAt
	}
	if t.State != previous || (v.Type == "PreToolUse" && !fileCue(t.State)) {
		t.HoldUntil = v.ReceivedAt.Add(600 * time.Millisecond)
	}
	newFileCue := v.Type == "FileChange" || v.Type == "item/fileChange/delta" ||
		((v.Type == "PostToolUse" || v.Type == "item/completed") && (v.Path != "" || strings.HasSuffix(v.Tool, "apply_patch") || v.Tool == "Edit" || v.Tool == "Write"))
	newReadCue := (v.Type == "PostToolUse" || v.Type == "item/completed") && reading
	if fileCue(t.State) && (newFileCue || newReadCue) {
		t.HoldUntil = v.ReceivedAt.Add(changeAfterglow)
		t.ResumeAt = t.HoldUntil
	}
	if t.State == Responding && (v.Type == "AgentMessage" || v.Type == "item/agentMessage/delta") {
		t.HoldUntil = v.ReceivedAt.Add(responseAfterglow)
	}
	if t.State != Failed {
		t.RecoverableError = false
	}
	t.ActiveTools, t.PendingApprovals = len(l.tools), len(l.waits)
	e.tasks[v.Thread] = t
	if v.ID != "" {
		e.seen[v.ID] = v.ReceivedAt
	}
	if len(e.seen) > 4096 {
		for key, at := range e.seen {
			if v.ReceivedAt.Sub(at) > 15*time.Minute {
				delete(e.seen, key)
			}
		}
	}
	if len(l.ended) > 2048 {
		for key, at := range l.ended {
			if v.ReceivedAt.Sub(at) > 15*time.Minute {
				delete(l.ended, key)
			}
		}
	}
	e.clockEvent(v)
	return true
}
func (e *Engine) Snapshot(now time.Time) []Task {
	e.mu.Lock()
	defer e.mu.Unlock()
	for id, t := range e.tasks {
		l := e.lives[id]
		if !t.ResumeAt.IsZero() && !now.Before(t.ResumeAt) && !t.RecoverableError {
			running(&t, l)
			t.ResumeAt = time.Time{}
			t.Since = now
		}
		if (t.State == Complete || t.State == Failed || t.State == Interrupted) && !t.DoneUntil.IsZero() && now.After(t.DoneUntil) {
			if t.State == Failed && t.RecoverableError {
				running(&t, l)
				t.DoneUntil = time.Time{}
				t.RecoverableError = false
				t.Since = now
			} else {
				delete(e.tasks, id)
				continue
			}
		}
		// Freshness is separate from completion. Do not invent a decision/finish.
		t.Stale = now.Sub(t.ReceivedAt) > 5*time.Minute
		if t.Stale && t.State != Approval {
			t.Confirmed = false
		}
		if t.Stale && now.Sub(t.ReceivedAt) > 24*time.Hour && len(l.waits) == 0 && len(l.tools) == 0 {
			delete(e.tasks, id)
			continue
		}
		e.tasks[id] = t
	}
	for id, l := range e.lives {
		if _, active := e.tasks[id]; !active && now.Sub(l.touched) > 24*time.Hour {
			delete(e.lives, id)
		}
	}
	out := make([]Task, 0, len(e.tasks))
	for _, t := range e.tasks {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := priority(out[i].State), priority(out[j].State)
		if a != b {
			return a < b
		}
		return out[i].Thread < out[j].Thread
	})
	return out
}
func priority(k Kind) int {
	switch k {
	case Approval:
		return 0
	case Failed, Interrupted:
		return 1
	case Editing, Coding, Reading:
		return 2
	case Tool:
		return 3
	case Responding:
		return 4
	case Thinking:
		return 5
	case Complete:
		return 6
	}
	return 7
}
