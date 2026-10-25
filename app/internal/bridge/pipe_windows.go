//go:build windows

package bridge

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"aula-codex-rgb-lab/internal/events"
	"aula-codex-rgb-lab/internal/status"
	"github.com/Microsoft/go-winio"
)

const PipeName = `\\.\pipe\AulaCodexRGB`

type rawEvent map[string]any

func endpoint() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", os.ErrNotExist
	}
	dir := filepath.Join(base, "AulaCodexRgb")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "events.jsonl"), nil
}
func Serve(ctx context.Context, e *status.Engine, errors chan<- error) error {
	listener, err := winio.ListenPipe(PipeName, &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;OW)(A;;GA;;;SY)", InputBufferSize: 4096, OutputBufferSize: 4096})
	if err != nil {
		return err
	}
	go func() { <-ctx.Done(); _ = listener.Close() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			select {
			case errors <- err:
			default:
			}
			time.Sleep(200 * time.Millisecond)
			continue
		}
		go handle(conn, e, errors)
	}
}
func handle(conn net.Conn, e *status.Engine, errors chan<- error) {
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(900 * time.Millisecond))
	dec := json.NewDecoder(io.LimitReader(conn, 1<<20))
	var raw rawEvent
	if err := dec.Decode(&raw); err != nil {
		return
	}
	if raw["query"] == "lcd_status" {
		_ = conn.SetWriteDeadline(time.Now().Add(500 * time.Millisecond))
		_ = json.NewEncoder(conn).Encode(e.Clock().Status(time.Now()))
		return
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return
	}
	ev, ok := events.Decode(b, "")
	if !ok {
		return
	}
	events.AppendMetadata(ev, e.Apply(ev))
}
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }
