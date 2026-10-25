//go:build windows

package main

import (
	"encoding/json"
	"errors"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"time"
)

const targetFPS = 10

type prefs struct {
	Brightness uint8 `json:"brightness"`
	FPS        int   `json:"fps"`
	Paused     bool  `json:"paused"`
}
type controlSignal struct {
	Action string    `json:"action"`
	At     time.Time `json:"at"`
}
type appStatus struct {
	State   string    `json:"state"`
	Updated time.Time `json:"updated"`
}

func userData() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		return "", errors.New("LOCALAPPDATA is unavailable")
	}
	dir := filepath.Join(base, "AulaCodexRgb")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

func loadPrefs(path string) prefs {
	p := prefs{Brightness: 65, FPS: targetFPS}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &p)
	}
	if p.Brightness < 5 || p.Brightness > 80 {
		p.Brightness = 65
	}
	p.FPS = targetFPS
	return p
}
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
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
