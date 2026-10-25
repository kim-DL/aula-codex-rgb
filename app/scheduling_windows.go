//go:build windows && rgb_gui

package main

import "golang.org/x/sys/windows"

// Shell/task launchers can pass BelowNormal to a child. Packet pacing depends
// on promptly waking after firmware waits. Restore the usual application class
// only when inherited below it; never elevate above Normal or alter power policy.
func needsNormalPriority(priority uint32) bool {
	return priority == windows.BELOW_NORMAL_PRIORITY_CLASS || priority == windows.IDLE_PRIORITY_CLASS
}

func normalizeScheduling() (before, after uint32, err error) {
	process := windows.CurrentProcess()
	before, err = windows.GetPriorityClass(process)
	if err != nil {
		return
	}
	if needsNormalPriority(before) {
		err = windows.SetPriorityClass(process, windows.NORMAL_PRIORITY_CLASS)
		if err != nil {
			return
		}
	}
	after, err = windows.GetPriorityClass(process)
	return
}
