package main

import (
	"aula-codex-rgb-lab/internal/events"
	"aula-codex-rgb-lab/internal/status"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

func main() {
	cutoff, _ := time.Parse(time.RFC3339, "2026-10-16T12:34:33Z")
	for _, path := range os.Args[1:] {
		f, err := os.Open(path)
		if err != nil {
			panic(err)
		}
		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 4096), 8<<20)
		engine := status.New()
		thread := ""
		now := time.Now()
		terminal := []any{}
		for scanner.Scan() {
			var root struct {
				Type    string `json:"type"`
				Payload struct {
					ID string `json:"id"`
				} `json:"payload"`
			}
			if json.Unmarshal(scanner.Bytes(), &root) == nil && root.Type == "session_meta" {
				thread = root.Payload.ID
			}
			ev, ok := events.DecodeForSession(scanner.Bytes(), path, thread)
			if !ok || ev.At.Before(cutoff) {
				continue
			}
			// Reconstruct lifecycle and retention at original event time, not replay time.
			ev.ReceivedAt = ev.At
			accepted := engine.Apply(ev)
			if ev.Type == "task_complete" || ev.Type == "UserPromptSubmit" || ev.Type == "Stop" {
				terminal = append(terminal, map[string]any{"type": ev.Type, "turn": ev.Turn, "at": ev.At, "accepted": accepted, "tasks": engine.Snapshot(ev.At)})
			}
		}
		if scanner.Err() != nil {
			panic(scanner.Err())
		}
		f.Close()
		b, _ := json.Marshal(map[string]any{"thread": thread, "lifecycle": terminal, "active": engine.Snapshot(now)})
		fmt.Println(string(b))
	}
}
