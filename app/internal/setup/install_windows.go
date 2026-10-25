//go:build windows

package setup

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

var hookEvents = []string{"UserPromptSubmit", "PreToolUse", "PostToolUse", "PermissionRequest", "Stop", "Interrupt", "SubagentStart", "SubagentStop", "SessionStart", "SessionEnd"}

type handler struct {
	Type           string `json:"type"`
	Command        string `json:"command"`
	CommandWindows string `json:"commandWindows,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
}
type group struct {
	Hooks []handler `json:"hooks"`
}

func configPath() (string, error) {
	base := os.Getenv("USERPROFILE")
	if base == "" {
		base, _ = os.UserHomeDir()
	}
	if base == "" {
		return "", fmt.Errorf("user profile path is unavailable")
	}
	return filepath.Join(base, ".codex", "hooks.json"), nil
}
func command(exe string) string {
	return "& '" + strings.ReplaceAll(exe, "'", "''") + "' hook"
}
func isOurs(h handler, exe string) bool {
	legacy := fmt.Sprintf("\"%s\" hook", strings.ReplaceAll(exe, `"`, `\"`))
	for _, candidate := range []string{h.Command, h.CommandWindows} {
		for _, want := range []string{command(exe), legacy} {
			if strings.EqualFold(strings.TrimSpace(candidate), want) {
				return true
			}
		}
	}
	return false
}

// Keep unknown handler and group fields, matcher conditions, and other hooks
// intact. Typed re-marshalling would silently strip fields we do not model.
type hookObject map[string]json.RawMessage
type hookDocument map[string][]hookObject

func jsonValue(v any) json.RawMessage { b, _ := json.Marshal(v); return b }
func ownHandler(raw hookObject, exe string) bool {
	var h handler
	b, _ := json.Marshal(raw)
	if json.Unmarshal(b, &h) != nil {
		return false
	}
	if h.CommandWindows == "" {
		_ = json.Unmarshal(raw["command_windows"], &h.CommandWindows)
	}
	return h.Type == "command" && isOurs(h, exe)
}
func mergeHooks(root map[string]json.RawMessage, exe string) error {
	hooks := hookDocument{}
	if raw := root["hooks"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return fmt.Errorf("existing hook definitions are invalid and were left untouched: %w", err)
		}
	}
	cmd := command(exe)
	for _, name := range hookEvents {
		found := false
		for _, g := range hooks[name] {
			var handlers []hookObject
			if err := json.Unmarshal(g["hooks"], &handlers); err != nil {
				return fmt.Errorf("invalid %s handlers: %w", name, err)
			}
			for _, h := range handlers {
				if ownHandler(h, exe) {
					found = true
					h["command"], h["commandWindows"] = jsonValue(cmd), jsonValue(cmd)
					delete(h, "command_windows")
				}
			}
			g["hooks"] = jsonValue(handlers)
		}
		if !found {
			hooks[name] = append(hooks[name], hookObject{"hooks": jsonValue([]handler{{Type: "command", Command: cmd, CommandWindows: cmd, Timeout: 2}})})
		}
	}
	root["hooks"], _ = json.Marshal(hooks)
	return nil
}
func removeHooks(root map[string]json.RawMessage, exe string) error {
	if len(root["hooks"]) == 0 {
		return nil
	}
	hooks := hookDocument{}
	if err := json.Unmarshal(root["hooks"], &hooks); err != nil {
		return err
	}
	for name, groups := range hooks {
		kept := groups[:0]
		for _, g := range groups {
			var handlers []hookObject
			if err := json.Unmarshal(g["hooks"], &handlers); err != nil {
				return err
			}
			retained := handlers[:0]
			for _, h := range handlers {
				if !ownHandler(h, exe) {
					retained = append(retained, h)
				}
			}
			g["hooks"] = jsonValue(retained)
			if len(retained) > 0 {
				kept = append(kept, g)
			}
		}
		if len(kept) == 0 {
			delete(hooks, name)
		} else {
			hooks[name] = kept
		}
	}
	root["hooks"], _ = json.Marshal(hooks)
	return nil
}
func Install(exe string) error {
	if _, err := os.Stat(exe); err != nil {
		return fmt.Errorf("CLI executable is unavailable: %w", err)
	}
	background := filepath.Join(filepath.Dir(exe), "aula-codex-rgb-background.exe")
	if _, err := os.Stat(background); err != nil {
		return fmt.Errorf("windowless tray executable is unavailable: build both executables first: %w", err)
	}
	p, err := configPath()
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	root := map[string]json.RawMessage{}
	if b, e := os.ReadFile(p); e == nil {
		if e = json.Unmarshal(b, &root); e != nil {
			return fmt.Errorf("existing hooks.json is invalid and was left untouched: %w", e)
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if err = mergeHooks(root, exe); err != nil {
		return err
	}
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	if _, e := os.Stat(p); e == nil {
		b, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		backup := p + ".backup-" + time.Now().Format("20060102-150405.000000000")
		if e = os.WriteFile(backup, b, 0600); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if err = writeAtomic(p, data, 0600); err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("AulaCodexRgb", fmt.Sprintf("\"%s\" run --background", strings.ReplaceAll(background, `"`, `\"`)))
}

func Uninstall(exe string) error {
	p, err := configPath()
	if err != nil {
		return err
	}
	if b, e := os.ReadFile(p); e == nil {
		var root map[string]json.RawMessage
		if e = json.Unmarshal(b, &root); e != nil {
			return e
		}
		if e = removeHooks(root, exe); e != nil {
			return e
		}
		b, e = json.MarshalIndent(root, "", "  ")
		if e != nil {
			return e
		}
		if e = writeAtomic(p, b, 0600); e != nil {
			return e
		}
	} else if !os.IsNotExist(e) {
		return e
	}
	if k, e := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE); e == nil {
		_ = k.DeleteValue("AulaCodexRgb")
		_ = k.Close()
	}
	return nil
}
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	defer os.Remove(tmp)
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	src, err := windows.UTF16PtrFromString(tmp)
	if err != nil {
		return err
	}
	dst, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(src, dst, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
}
