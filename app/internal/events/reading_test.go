package events

import (
	"aula-codex-rgb-lab/internal/status"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadShellConservativeBoundaries(t *testing.T) {
	for _, cmd := range []string{
		`Get-Content -LiteralPath 'notes.md'`,
		`$lab='C:\Lab'; Get-Content "$lab\notes.md" -TotalCount 25`,
		`$p=Join-Path 'C:\Lab' 'notes.md'; Get-Content $p | Select-Object -First 20`,
		`cat app.go | head -40`, `rg -n 'hello' src`,
		`Get-Content 'notes.md' | Select-Object -First 10; Get-Content 'other.txt'`,
	} {
		if !readShell(cmd) {
			t.Errorf("read rejected: %s", cmd)
		}
	}
	for _, cmd := range []string{
		``, `Get-ChildItem`, `rg --files src`, `Get-FileHash notes.md`,
		`Get-Content notes.md; Set-Content notes.md new`, `cat a > b`,
		`Get-Content notes.md; go test ./...`, `python script.py`,
		`Get-Content "$(Remove-Item a)"`, `Get-Content a | ForEach-Object { & $_ }`,
		`$p=Invoke-WebRequest 'x'; Get-Content $p`, `Get-Content 'unclosed`,
		`Get-Content "$env:LOCALAPPDATA\AulaCodexRgb\health.json"`,
		`rg --pre 'external-command' pattern file`,
	} {
		if readShell(cmd) {
			t.Errorf("ambiguous command accepted: %s", cmd)
		}
	}
}
func TestReadWrapperUnderstandsOnlySupportedLiteralCalls(t *testing.T) {
	for _, code := range []string{
		`text((await tools.exec_command({cmd:"Get-Content -LiteralPath 'notes.md'"})).output)`,
		`const r=await tools.exec_command({"cmd":"cat 'app.go'"});text(r.output);`,
		`const r=await tools.mcp__docs__read_document({path:"notes.docx"});text(r);`,
		`await Promise.all([tools.exec_command({cmd:"cat a"}),tools.exec_command({cmd:'cat b'})]);`,
	} {
		if !readWrapper(code) {
			t.Errorf("read wrapper rejected: %s", code)
		}
	}
	for _, code := range []string{
		`text("tools.exec_command({cmd:'cat a'})")`,
		`await tools.exec_command({cmd:"cat a"}); await tools.exec_command({cmd:"go test ./..."});`,
		`await tools.exec_command({cmd:command});`,
		`await tools.exec_command({cmd:"cat file"+suffix});`,
		"await tools.exec_command({cmd:`cat ${file}`});",
		`if(false) await tools.exec_command({cmd:"cat a"});`,
		`await tools.exec_command({cmd:"cat a"});await tools.clock__curr_time({});`,
	} {
		if readWrapper(code) {
			t.Errorf("ambiguous wrapper accepted: %s", code)
		}
	}
}
func TestDecodeReadingRemovesInputsAndKeepsConfidence(t *testing.T) {
	for _, test := range []struct {
		tool  string
		input map[string]any
		exact bool
	}{
		{"Read", map[string]any{"file_path": "notes.md", "secret": "PRIVATE-INPUT"}, true},
		{"exec_command", map[string]any{"cmd": "Get-Content 'PRIVATE-FILE.md'"}, false},
		{"exec", map[string]any{"code": `text((await tools.exec_command({cmd:"cat PRIVATE-FILE.md"})).output)`}, false},
	} {
		raw, _ := json.Marshal(map[string]any{"session_id": "s", "turn_id": "t", "hook_event_name": "PreToolUse", "tool_name": test.tool, "tool_use_id": "c", "tool_input": test.input})
		e, ok := Decode(raw, "")
		if !ok || e.Activity != status.Reading || e.ActivityConfirmed != test.exact {
			t.Fatalf("activity %s: %+v", test.tool, e)
		}
		b, _ := json.Marshal(e)
		if strings.Contains(string(b), "PRIVATE-INPUT") || strings.Contains(string(b), "cmd") || strings.Contains(string(b), "Get-Content") {
			t.Fatal("private input retained")
		}
	}
	code := `text((await tools.exec_command({cmd:"Get-Content 'readme.md'"})).output)`
	raw, _ := json.Marshal(map[string]any{"type": "response_item", "payload": map[string]any{"type": "custom_tool_call", "name": "exec", "call_id": "c", "input": code}})
	e, ok := DecodeForSession(raw, "rollout.jsonl", "s")
	if !ok || e.Activity != status.Reading || e.ActivityConfirmed {
		t.Fatal(e)
	}
}
