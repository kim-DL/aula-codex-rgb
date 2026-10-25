// Preview is an offline development tool; it is not linked into the tray app.
package main

import (
	"aula-codex-rgb-lab/internal/status"
	"aula-codex-rgb-lab/internal/visual"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed index.html
var template string

func main() {
	dir := "../preview"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	type scene struct {
		ID, Label string
		Kind      status.Kind
	}
	scenes := []scene{{"thinking", "생각 중 · 생명 파동", status.Thinking}, {"tool", "도구 실행", status.Tool}, {"editing", "문서 수정", status.Editing}, {"approval", "승인 대기", status.Approval}, {"coding", "코드 수정", status.Coding}, {"responding", "응답 활동", status.Responding}, {"failed", "오류", status.Failed}, {"complete", "완료 → 편안한 대기", status.Complete}, {"idle", "대기 · 샌드빛 + 흰빛 유영", status.Idle}}
	keys := visual.Layout()
	frames := map[string][][]uint32{}
	start := time.Unix(0, 0)
	for _, s := range scenes {
		r := visual.New()
		r.Start = start
		tasks := []status.Task{{Thread: "preview", State: s.Kind, Updated: start}}
		if s.Kind == status.Idle {
			tasks = nil
		}
		count := 120
		if s.Kind == status.Thinking {
			count = 400
		}
		if s.Kind == status.Idle {
			count = 400
		}
		for n := 0; n < count; n++ {
			f := r.Render(tasks, start.Add(time.Duration(n)*100*time.Millisecond))
			colors := make([]uint32, len(keys))
			for i, k := range keys {
				c := f[k.Index]
				colors[i] = uint32(c[0])<<16 | uint32(c[1])<<8 | uint32(c[2])
			}
			frames[s.ID] = append(frames[s.ID], colors)
		}
	}
	data, _ := json.Marshal(map[string]any{"fps": 10, "brightness": 65, "keys": keys, "scenes": scenes, "frames": frames})
	if err := os.WriteFile(filepath.Join(dir, "frames.json"), data, 0600); err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(strings.Replace(template, "/*DATA*/", string(data), 1)), 0600); err != nil {
		panic(err)
	}
	fmt.Println(filepath.Join(dir, "index.html"))
}
