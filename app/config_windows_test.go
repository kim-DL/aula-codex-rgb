//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCurrentModeIsDefaultAndOldSettingsMigrate(t *testing.T) {
	p := loadPrefs(filepath.Join(t.TempDir(), "missing.json"))
	if p.Brightness != 65 || p.FPS != 10 || p.Paused {
		t.Fatal(p)
	}
	path := filepath.Join(t.TempDir(), "settings.json")
	os.WriteFile(path, []byte(`{"brightness":65,"fps":15,"effect":"legacy","thinking_effect":"profile","connections":"fade","paused":true}`), 0600)
	p = loadPrefs(path)
	if p.Brightness != 65 || p.FPS != 10 || !p.Paused {
		t.Fatal(p)
	}
}
