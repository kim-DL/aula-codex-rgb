//go:build windows && rgb_gui

package main

import (
	"golang.org/x/sys/windows"
	"testing"
)

func TestNormalizeOnlyInheritedLowPriority(t *testing.T) {
	for _, priority := range []uint32{windows.IDLE_PRIORITY_CLASS, windows.BELOW_NORMAL_PRIORITY_CLASS} {
		if !needsNormalPriority(priority) {
			t.Fatalf("low priority %x remains inherited", priority)
		}
	}
	for _, priority := range []uint32{windows.NORMAL_PRIORITY_CLASS, windows.ABOVE_NORMAL_PRIORITY_CLASS, windows.HIGH_PRIORITY_CLASS, windows.REALTIME_PRIORITY_CLASS, 0} {
		if needsNormalPriority(priority) {
			t.Fatalf("unrelated priority %x changed", priority)
		}
	}
}
