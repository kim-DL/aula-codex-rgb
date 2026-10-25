//go:build windows && !rgb_gui

package main

import (
	"aula-codex-rgb-lab/internal/bridge"
	"aula-codex-rgb-lab/internal/events"
	"aula-codex-rgb-lab/internal/instance"
	"aula-codex-rgb-lab/internal/setup"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Microsoft/go-winio"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func hookMetadata(raw []byte) ([]byte, bool, error) {
	ev, ok := events.Decode(raw, "")
	if !ok {
		return nil, false, nil
	}
	metadata := map[string]any{"event_id": ev.ID, "hook_event_name": ev.Type, "session_id": ev.Thread, "turn_id": ev.Turn, "tool_name": ev.Tool, "file_path": ev.Path, "timestamp": ev.At}
	metadata["tool_use_id"] = ev.Call
	metadata["temporary_session"] = ev.Temporary
	if ev.Activity == "reading" {
		metadata["activity"], metadata["activity_confirmed"] = ev.Activity, ev.ActivityConfirmed
	}
	if ev.Error {
		metadata["error"] = true
	}
	data, err := json.Marshal(metadata)
	return data, true, err
}
func sendHook(raw []byte) error {
	data, ok, err := hookMetadata(raw)
	if err != nil || !ok {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	conn, err := winio.DialPipeContext(ctx, bridge.PipeName)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond))
	_, err = conn.Write(data)
	return err
}
func sendControl(action string) error {
	dir, err := userData()
	if err != nil {
		return err
	}
	running, err := instance.IsRunning()
	if err != nil {
		return err
	}
	if !running {
		return errors.New("AULA RGB is not running")
	}
	b, _ := json.Marshal(controlSignal{Action: action, At: time.Now().UTC()})
	if err := atomicWrite(filepath.Join(dir, "control.json"), b, 0600); err != nil {
		return err
	}
	fmt.Printf("%s 요청을 보냈습니다.\n", action)
	return nil
}
func main() {
	if len(os.Args) < 2 {
		fmt.Println("aula-codex-rgb {run|hook|status|lcd-status|pause|resume|stop|install|uninstall}")
		return
	}
	var err error
	switch os.Args[1] {
	case "hook":
		b, e := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
		if e == nil {
			_ = sendHook(b)
		}
		return
	case "run":
		running, _ := instance.IsRunning()
		if running {
			return
		}
		exe, _ := os.Executable()
		cmd := exec.Command(filepath.Join(filepath.Dir(exe), "aula-codex-rgb-background.exe"), "run")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		err = cmd.Start()
		if err == nil {
			_ = cmd.Process.Release()
		}
	case "pause", "resume", "stop":
		err = sendControl(os.Args[1])
	case "lcd-status":
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		var conn net.Conn
		conn, err = winio.DialPipeContext(ctx, bridge.PipeName)
		if err == nil {
			defer conn.Close()
			_ = conn.SetDeadline(time.Now().Add(time.Second))
			err = json.NewEncoder(conn).Encode(map[string]string{"query": "lcd_status"})
			if err == nil {
				_, err = io.Copy(os.Stdout, io.LimitReader(conn, 8192))
			}
		}
	case "status":
		dir, e := userData()
		if e != nil {
			err = e
			break
		}
		p := loadPrefs(filepath.Join(dir, "settings.json"))
		var s appStatus
		b, _ := os.ReadFile(filepath.Join(dir, "status.json"))
		_ = json.Unmarshal(b, &s)
		running, _ := instance.IsRunning()
		if !running {
			s.State = "실행 중 아님"
		}
		fmt.Printf("%s · 생명 파동 / 집중 / 흐름 연결 · 밝기 %d%% · %d fps\n", s.State, p.Brightness, targetFPS)
	case "install", "uninstall":
		exe, _ := os.Executable()
		if os.Args[1] == "install" {
			err = setup.Install(exe)
		} else {
			err = setup.Uninstall(exe)
		}
	default:
		err = fmt.Errorf("지원하지 않는 명령: %s", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
