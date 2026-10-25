//go:build windows

package main

import (
	"errors"
	"reflect"
	"testing"
)

// No real registry, hook file, resident process, or HID is touched by these tests.
func TestDesktopActionsPreserveBoundaries(t *testing.T) {
	background := `C:\한글 폴더 & test\bin\aula-codex-rgb-background.exe`
	for _, tc := range []struct {
		name, action, registration, fail string
		running, wantError               bool
		want                             []string
	}{
		{"install and launch", "install", "", "", false, false, []string{"install", "start"}},
		{"repeat install", "install", `"` + background + `" run --background`, "", false, false, []string{"install", "start"}},
		{"run only", "run", "", "", false, false, []string{"start"}},
		{"remove after graceful stop", "uninstall", "", "", true, false, []string{"running", "stop", "wait", "uninstall"}},
		{"remove already stopped", "uninstall", "", "", false, false, []string{"running", "uninstall"}},
		{"install failure never launches", "install", "", "install", false, true, []string{"install"}},
		{"stop failure preserves registration", "uninstall", "", "stop", true, true, []string{"running", "stop"}},
		{"timeout preserves registration", "uninstall", "", "wait", true, true, []string{"running", "stop", "wait"}},
		{"foreign install", "install", `"C:\Other\app.exe" run`, "", false, true, nil},
		{"foreign uninstall", "uninstall", `"C:\Other\app.exe" run`, "", true, true, nil},
		{"unknown action", "delete-all", "", "", false, true, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			step := func(name string) func() error {
				return func() error {
					got = append(got, name)
					if tc.fail == name {
						return errors.New("fixture failure")
					}
					return nil
				}
			}
			op := operations{
				registration: func() (string, error) { return tc.registration, nil },
				install:      step("install"), uninstall: step("uninstall"), start: step("start"), stop: step("stop"), waitStopped: step("wait"),
				running: func() (bool, error) { got = append(got, "running"); return tc.running, nil },
			}
			err := perform(tc.action, background, op)
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("actions=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestRegistrationReadFailureDoesNotMutate(t *testing.T) {
	op := operations{registration: func() (string, error) { return "", errors.New("access denied") }}
	if err := perform("install", `C:\App\background.exe`, op); err == nil {
		t.Fatal("registration read failure must stop installation")
	}
}
