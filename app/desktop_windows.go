//go:build windows && rgb_gui

package main

import (
	"golang.org/x/sys/windows"
	"strings"
	"time"
	"unsafe"
)

// The installed desktop is ChatGPT.exe inside OpenAI.Codex's MSIX package.
// A standalone codex.exe CLI is deliberately insufficient to enable the LCD.
func desktopRunning() bool {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if !strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "ChatGPT.exe") {
			continue
		}
		p, e := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, entry.ProcessID)
		if e != nil {
			continue
		}
		buf := make([]uint16, 32768)
		size := uint32(len(buf))
		e = windows.QueryFullProcessImageName(p, 0, &buf[0], &size)
		windows.CloseHandle(p)
		if e == nil && strings.Contains(strings.ToLower(windows.UTF16ToString(buf[:size])), `\windowsapps\openai.codex_`) {
			return true
		}
	}
	return false
}

func (a *application) desktopLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.engine.Clock().SetEnabled(desktopRunning())
		}
	}
}
