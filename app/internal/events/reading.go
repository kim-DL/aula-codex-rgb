package events

import (
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Classification is deliberately bounded and fail-closed. Inputs are examined
// in memory once, never executed or retained. Only a reading enum leaves here.
func toolBase(name string) string {
	name = strings.ToLower(name)
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	if i := strings.LastIndex(name, "__"); i >= 0 {
		name = name[i+2:]
	}
	return name
}
func explicitReader(name string) bool {
	switch toolBase(name) {
	case "read", "read_file", "read_text_file", "read_multiple_files", "read_document", "read_page", "read_page_reference", "read_thread", "read_pdf", "view_image", "extract_text", "get_document_text", "readfile", "readdocument":
		return true
	}
	return false
}
func readingActivity(tool string, root map[string]any) (reading, confirmed bool) {
	if explicitReader(tool) {
		return true, true
	}
	input := nested(root, "tool_input")
	if len(input) == 0 {
		input = nested(root, "arguments")
	}
	switch toolBase(tool) {
	case "exec_command", "shell_command", "run_command", "bash", "powershell", "execute_shell_command":
		command := first(input, "cmd", "command", "script")
		if command == "" {
			command = first(root, "cmd", "command")
		}
		return readShell(command), false
	case "exec":
		code := first(input, "code", "input")
		if code == "" {
			code = first(root, "input")
		}
		if code == "" {
			code, _ = root["tool_input"].(string)
		}
		return readWrapper(code), false
	}
	return false, false
}

var assignment = regexp.MustCompile(`^\$[A-Za-z_][A-Za-z0-9_]*\s*=\s*`)
var commandLiteral = regexp.MustCompile(`(?:^|[,{]\s*)(?:"cmd"|cmd|"command"|command)\s*:\s*("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')`)

func readShell(command string) bool {
	if len(command) == 0 || len(command) > 65536 || strings.Contains(command, "$(") {
		return false
	}
	lower := strings.ToLower(command)
	if strings.Contains(lower, "aulacodexrgb") {
		for _, name := range []string{"health.json", "control.json", "settings.json", "status.json", "hardware-events.jsonl", "light-events.jsonl"} {
			if strings.Contains(lower, name) {
				return false
			}
		}
	}
	var parts []string
	start := 0
	var quote byte
	for i := 0; i < len(command); i++ {
		c := command[i]
		if quote != 0 {
			if c == quote {
				if i+1 < len(command) && command[i+1] == quote {
					i++
					continue
				}
				quote = 0
			} else if c == '`' && quote == '"' {
				i++
			}
			continue
		}
		switch c {
		case '\'', '"':
			quote = c
		case ';', '|', '\n', '\r':
			parts = append(parts, command[start:i])
			start = i + 1
		case '&', '>', '<', '{', '}', '(', ')', '`', '#':
			return false
		}
	}
	if quote != 0 {
		return false
	}
	parts = append(parts, command[start:])
	read := false
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		assigned := assignment.MatchString(part)
		if assigned {
			part = strings.TrimSpace(assignment.ReplaceAllString(part, ""))
		}
		if assigned && len(part) > 1 && (part[0] == '\'' || part[0] == '"') && part[len(part)-1] == part[0] {
			continue
		}
		words := strings.Fields(part)
		if len(words) == 0 {
			return false
		}
		name := strings.ToLower(words[0])
		args := strings.TrimSpace(strings.TrimPrefix(part, words[0]))
		switch name {
		case "get-content", "cat", "type", "head", "tail":
			if args == "" {
				return false
			}
			read = true
		case "rg", "grep":
			if args == "" || strings.Contains(strings.ToLower(args), "--files") || strings.Contains(strings.ToLower(args), "--pre") || strings.Contains(args, " >") {
				return false
			}
			read = true
		case "select-string":
			if strings.Contains(strings.ToLower(args), "-path") || strings.Contains(strings.ToLower(args), "-literalpath") {
				read = true
			}
		case "join-path", "split-path":
			if !assigned {
				return false
			}
		case "select-object", "sort-object", "measure-object", "format-table", "format-list", "out-string", "convertfrom-json", "convertto-json":
			// Output shaping does not establish a read on its own.
		default:
			return false
		}
	}
	return read
}

func jsString(code string, at int) (value string, end int, ok bool) {
	q := code[at]
	if q != '\'' && q != '"' {
		return "", at, false
	}
	var b strings.Builder
	for i := at + 1; i < len(code); {
		if code[i] == q {
			return b.String(), i + 1, true
		}
		ch, _, tail, err := strconv.UnquoteChar(code[i:], q)
		if err != nil {
			return "", i, false
		}
		b.WriteRune(ch)
		i = len(code) - len(tail)
	}
	return "", len(code), false
}
func jsArguments(code string, open int) (string, int, bool) {
	depth := 1
	for i := open + 1; i < len(code); i++ {
		if code[i] == '\'' || code[i] == '"' {
			_, end, ok := jsString(code, i)
			if !ok {
				return "", i, false
			}
			i = end - 1
			continue
		}
		if code[i] == '`' {
			return "", i, false
		}
		if code[i] == '(' {
			depth++
		}
		if code[i] == ')' {
			depth--
			if depth == 0 {
				return code[open+1 : i], i + 1, true
			}
		}
	}
	return "", len(code), false
}
func readWrapper(code string) bool {
	if len(code) == 0 || len(code) > 65536 {
		return false
	}
	calls := 0
	for i := 0; i < len(code); {
		if code[i] == '\'' || code[i] == '"' {
			_, end, ok := jsString(code, i)
			if !ok {
				return false
			}
			i = end
			continue
		}
		if code[i] == '`' || code[i] == '?' {
			return false
		}
		if strings.HasPrefix(code[i:], "//") {
			end := strings.IndexByte(code[i:], '\n')
			if end < 0 {
				break
			}
			i += end + 1
			continue
		}
		if strings.HasPrefix(code[i:], "/*") {
			end := strings.Index(code[i+2:], "*/")
			if end < 0 {
				return false
			}
			i += end + 4
			continue
		}
		if strings.HasPrefix(code[i:], "tools.") {
			start := i + 6
			j := start
			for j < len(code) && (code[j] == '_' || code[j] >= 'a' && code[j] <= 'z' || code[j] >= 'A' && code[j] <= 'Z' || code[j] >= '0' && code[j] <= '9') {
				j++
			}
			name := code[start:j]
			for j < len(code) && strings.ContainsRune(" \t\r\n", rune(code[j])) {
				j++
			}
			if j >= len(code) || code[j] != '(' {
				return false
			}
			args, end, ok := jsArguments(code, j)
			if !ok {
				return false
			}
			if !explicitReader(name) {
				if toolBase(name) != "exec_command" && toolBase(name) != "shell_command" {
					return false
				}
				match := commandLiteral.FindStringSubmatch(args)
				if len(match) != 2 {
					return false
				}
				indices := commandLiteral.FindStringIndex(args)
				rest := strings.TrimSpace(args[indices[1]:])
				if rest == "" || (rest[0] != ',' && rest[0] != '}') {
					return false
				}
				command, _, ok := jsString(match[1], 0)
				if !ok || !readShell(command) {
					return false
				}
			}
			calls++
			i = end
			continue
		}
		if code[i] >= 'a' && code[i] <= 'z' || code[i] >= 'A' && code[i] <= 'Z' {
			start := i
			for i < len(code) && (code[i] == '_' || code[i] >= 'a' && code[i] <= 'z' || code[i] >= 'A' && code[i] <= 'Z') {
				i++
			}
			switch code[start:i] {
			case "if", "for", "while", "switch", "function", "eval":
				return false
			}
			continue
		}
		_, n := utf8.DecodeRuneInString(code[i:])
		i += n
	}
	return calls > 0
}
