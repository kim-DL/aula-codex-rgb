//go:build windows

package setup

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestMergeAndRemoveOnlyThisAppsCodexHooks(t *testing.T) {
	exe := `C:\Codex Lab\aula-codex-rgb.exe`
	other := `C:\Other\aula-codex-rgb.exe`
	initialHooks := map[string][]group{
		"UserPromptSubmit": {{Hooks: []handler{{Type: "command", Command: command(exe)}, {Type: "command", Command: "echo keep-user-hook"}}}},
		"Stop":             {{Hooks: []handler{{Type: "command", Command: command(other)}}}},
	}
	rawHooks, _ := json.Marshal(initialHooks)
	root := map[string]json.RawMessage{"hooks": rawHooks, "notify": json.RawMessage(`["preserve-me"]`)}
	if err := mergeHooks(root, exe); err != nil {
		t.Fatal(err)
	}
	if err := mergeHooks(root, exe); err != nil {
		t.Fatal(err)
	}
	var merged map[string][]group
	if err := json.Unmarshal(root["hooks"], &merged); err != nil {
		t.Fatal(err)
	}
	managed := 0
	for _, groups := range merged {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if h.Type == "command" && isOurs(h, exe) {
					managed++
				}
			}
		}
	}
	if managed != len(hookEvents) {
		t.Fatalf("managed hook count=%d, want %d", managed, len(hookEvents))
	}
	if !strings.Contains(string(root["notify"]), "preserve-me") {
		t.Fatal("unrelated Codex settings were not preserved")
	}
	if err := removeHooks(root, exe); err != nil {
		t.Fatal(err)
	}
	var removed map[string][]group
	if err := json.Unmarshal(root["hooks"], &removed); err != nil {
		t.Fatal(err)
	}
	for _, groups := range removed {
		for _, g := range groups {
			for _, h := range g.Hooks {
				if isOurs(h, exe) {
					t.Fatal("this app's hook survived uninstall")
				}
			}
		}
	}
	if len(removed["UserPromptSubmit"]) != 1 || len(removed["UserPromptSubmit"][0].Hooks) != 1 || removed["UserPromptSubmit"][0].Hooks[0].Command != "echo keep-user-hook" {
		t.Fatal("user hook was not preserved")
	}
	if len(removed["Stop"]) != 1 || len(removed["Stop"][0].Hooks) != 1 || removed["Stop"][0].Hooks[0].Command != command(other) {
		t.Fatal("different executable hook was removed")
	}
}

func TestMigrationKeepsMatcherAndUnrelatedHandlerOptions(t *testing.T) {
	exe := `C:\Codex Lab\aula-codex-rgb.exe`
	root := map[string]json.RawMessage{"hooks": json.RawMessage(`{"PreToolUse":[{"matcher":"^Bash$","customGroup":"keep","hooks":[{"type":"command","command":"echo user-hook","async":true,"statusMessage":"preserve","custom":"keep"},{"type":"command","command":"\"C:\\Codex Lab\\aula-codex-rgb.exe\" hook","command_windows":"\"C:\\Codex Lab\\aula-codex-rgb.exe\" hook","timeout":2}]}]}`)}
	var before hookDocument
	_ = json.Unmarshal(root["hooks"], &before)
	var original []hookObject
	_ = json.Unmarshal(before["PreToolUse"][0]["hooks"], &original)
	if err := mergeHooks(root, exe); err != nil {
		t.Fatal(err)
	}
	var after hookDocument
	_ = json.Unmarshal(root["hooks"], &after)
	group := after["PreToolUse"][0]
	if string(group["matcher"]) != `"^Bash$"` || string(group["customGroup"]) != `"keep"` {
		t.Fatal("group metadata changed")
	}
	var updated []hookObject
	_ = json.Unmarshal(group["hooks"], &updated)
	if !reflect.DeepEqual(updated[0], original[0]) {
		t.Fatal("unrelated handler options changed")
	}
	var cmd string
	_ = json.Unmarshal(updated[1]["commandWindows"], &cmd)
	if cmd != command(exe) || len(updated[1]["command_windows"]) != 0 {
		t.Fatal("Windows invocation was not migrated")
	}
	if err := removeHooks(root, exe); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal(root["hooks"], &after)
	_ = json.Unmarshal(after["PreToolUse"][0]["hooks"], &updated)
	if len(updated) != 1 || !reflect.DeepEqual(updated[0], original[0]) {
		t.Fatal("uninstall changed another handler")
	}
}
