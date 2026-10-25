//go:build windows

// Motion study; default is offline. Physical B demo requires --hardware-b.
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"aula-codex-rgb-lab/internal/visual"
)

func candidateColor(variant string, k visual.LayoutKey, sec float64) [3]uint8 {
	if variant == "sand-pearl-refined" || variant == "sand-ripple-refined" {
		return refinedColor(variant, k, sec)
	}
	x, y := k.X+k.W/2, k.Y+k.H/2
	if sec < 0 {
		sec = 0
	}
	if variant == "sand-pearl" || variant == "sand-ripple" {
		var alpha float64
		if variant == "sand-pearl" {
			cx := 11.75 - 9.5*math.Cos(2*math.Pi*sec/12)
			cy := 3.0 + .85*math.Sin(2*math.Pi*sec/8.3)
			tx := 11.75 - 9.5*math.Cos(2*math.Pi*(sec-.8)/12)
			ty := 3.0 + .85*math.Sin(2*math.Pi*(sec-.8)/8.3)
			head := math.Exp(-.5 * (math.Pow((x-cx)/2.1, 2) + math.Pow((y-cy)/2.7, 2)))
			tail := math.Exp(-.5 * (math.Pow((x-tx)/3.0, 2) + math.Pow((y-ty)/3.3, 2)))
			alpha = .94*head + .24*tail*(1-head)
		} else {
			phase := math.Mod(sec, 8)
			cx := 10.3 + 1.1*math.Sin(2*math.Pi*sec/23)
			cy := 3.0 + .4*math.Sin(2*math.Pi*sec/17)
			d := math.Hypot(x-cx, (y-cy)*1.45)
			radius := phase*3.6 - 5
			front := math.Exp(-.5 * math.Pow((d-radius)/1.65, 2))
			tail := math.Exp(-.5 * math.Pow((d-(radius-2.0))/2.1, 2))
			edge := math.Pow(math.Sin(math.Pi*phase/8), 2)
			alpha = (.94*front + .20*tail*(1-front)) * edge
		}
		alpha = math.Max(0, math.Min(1, alpha))
		sand, white := [3]uint8{26, 16, 7}, [3]uint8{72, 72, 72}
		var out [3]uint8
		for ch := range out {
			out[ch] = uint8(math.Round(float64(sand[ch]) + (float64(white[ch])-float64(sand[ch]))*alpha))
		}
		return out
	}
	var level float64
	switch variant {
	case "pulse-balanced":
		// Preserve the old sand floor, raise only the moving soft field.
		cx := 11.3 + 6.0*math.Sin(2*math.Pi*sec/23) + .8*math.Sin(2*math.Pi*sec/37)
		cy := 3.1 + .65*math.Sin(2*math.Pi*sec/19)
		tx := 11.3 + 6.0*math.Sin(2*math.Pi*(sec-2)/23) + .8*math.Sin(2*math.Pi*(sec-2)/37)
		ty := 3.1 + .65*math.Sin(2*math.Pi*(sec-2)/19)
		d := math.Hypot((x-cx)/3.4, (y-cy)/3.8)
		core := math.Exp(-.5 * d * d)
		tail := math.Exp(-.5 * (math.Pow((x-tx)/5.3, 2) + math.Pow((y-ty)/4.5, 2)))
		pulse := .5 + .5*math.Sin(2*math.Pi*sec/7.8-d*.7)
		shoulder := .5 + .5*math.Sin(2*math.Pi*sec/10.6-d*2.0-.8)
		level = .43 + .40*core*(.86+.14*pulse) + .13*tail + .025*tail*shoulder
	case "pulse-defined":
		// A clearly shaped soft band over a darker sand floor, same RGB peak.
		phase := 2 * math.Pi * sec / 22
		cx := 11.75 - 9.5*math.Cos(phase)
		tx := 11.75 - 9.5*math.Cos(2*math.Pi*(sec-1.6)/22)
		cy := 3.1 + .45*math.Sin(2*math.Pi*sec/29)
		core := math.Exp(-.5 * (math.Pow((x-cx)/2.1, 2) + math.Pow((y-cy)/5.2, 2)))
		tail := math.Exp(-.5 * (math.Pow((x-tx)/3.8, 2) + math.Pow((y-cy)/5.6, 2)))
		lift := .94 + .06*math.Sin(2*math.Pi*sec/8.4)
		level = .12 + .80*core*lift + .07*tail
	case "pulse-visible":
		// A smaller moving field with a dimmer warm background, same peak/color.
		cx := 11.2 + 6.7*math.Sin(2*math.Pi*sec/15)
		cy := 3.0 + .6*math.Sin(2*math.Pi*sec/21)
		tx := 11.2 + 6.7*math.Sin(2*math.Pi*(sec-1.35)/15)
		ty := 3.0 + .6*math.Sin(2*math.Pi*(sec-1.35)/21)
		d := math.Hypot((x-cx)/3.4, (y-cy)/3.6)
		core := math.Exp(-.5 * d * d)
		tail := math.Exp(-.5 * (math.Pow((x-tx)/5.0, 2) + math.Pow((y-ty)/4.2, 2)))
		pulse := .5 + .5*math.Sin(2*math.Pi*(sec/6.8-d*.12))
		echo := .5 + .5*math.Sin(2*math.Pi*(sec/6.8-d*.12)-.8)
		level = .40 + .40*core*(.83+.17*pulse) + .16*tail*(.65+.35*echo)
	case "drift":
		// Two broad fields drift at different periods, never reversing abruptly.
		cx := 11.5 + 7.0*math.Sin(2*math.Pi*sec/23)
		cy := 3.0 + .7*math.Sin(2*math.Pi*sec/31)
		field := math.Exp(-.5 * (math.Pow((x-cx)/5.5, 2) + math.Pow((y-cy)/3.4, 2)))
		current := .5 + .5*math.Sin(2*math.Pi*sec/11-x*.23-y*.11)
		level = .77 + .12*field + .11*current
	default:
		// Moving nucleus with a broad, delayed shoulder, no isolated sparkles.
		cx := 11.2 + 4.8*math.Sin(2*math.Pi*sec/19)
		cy := 3.0 + .6*math.Sin(2*math.Pi*sec/27)
		d := math.Hypot((x-cx)/7.2, (y-cy)/4.0)
		core := math.Exp(-.5 * d * d)
		phase := 2 * math.Pi * (sec/7.4 - d*.15)
		pulse := math.Pow(.5+.5*math.Sin(phase), 2)
		echo := math.Pow(.5+.5*math.Sin(phase-.85), 2)
		tide := .5 + .5*math.Sin(2*math.Pi*sec/29-x*.13)
		level = .76 + .05*tide + .05*core + .105*pulse + .035*echo
	}
	if variant == "pulse-balanced" {
		level = math.Max(.43, math.Min(1, level))
	} else if variant == "pulse-defined" {
		level = math.Max(.12, math.Min(1, level))
	} else if variant == "pulse-visible" {
		level = math.Max(.40, math.Min(1, level))
	} else {
		level = math.Max(.76, math.Min(1, level))
	}
	var out [3]uint8
	base := [3]uint8{44, 28, 12}
	if variant == "pulse-balanced" {
		base = [3]uint8{88, 56, 24}
	}
	for ch, c := range base {
		out[ch] = uint8(uint16(uint8(float64(c)*level)) * 65 / 100)
	}
	return out
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--hardware-b" {
		runHardware(os.Args[2:])
		return
	}
	dir := "idle-study-preview"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	keys := visual.Layout()
	type scene struct{ ID, Label string }
	scenes := []scene{{"sand-pearl", "이전 · 흰빛 유영"}, {"sand-pearl-refined", "개선 · 부드러운 흰빛 유영"}, {"sand-ripple", "이전 · 퍼지는 물결"}, {"sand-ripple-refined", "개선 · 여운 있는 흰빛 물결"}}
	frames := map[string][][]uint32{}
	metrics := map[string]any{}
	for _, s := range scenes {
		r := visual.New()
		r.Start = time.Unix(0, 0)
		var low, high [3]uint8
		low = [3]uint8{255, 255, 255}
		maxStep, maxSpread := 0, 0
		var previous [][3]uint8
		for n := 0; n < 600; n++ {
			sec := float64(n) / 10
			colors := make([]uint32, len(keys))
			raw := make([][3]uint8, len(keys))
			frame := r.Render(nil, r.Start.Add(time.Duration(n)*100*time.Millisecond))
			spatialLo, spatialHi := 255, 0
			for i, k := range keys {
				c := frame[k.Index]
				if s.ID != "current" {
					c = candidateColor(s.ID, k, sec)
				}
				raw[i] = c
				colors[i] = uint32(c[0])<<16 | uint32(c[1])<<8 | uint32(c[2])
				for ch, v := range c {
					low[ch] = min(low[ch], v)
					high[ch] = max(high[ch], v)
					if previous != nil {
						maxStep = max(maxStep, int(math.Abs(float64(v)-float64(previous[i][ch]))))
					}
				}
				spatialLo = min(spatialLo, int(c[0]))
				spatialHi = max(spatialHi, int(c[0]))
			}
			maxSpread = max(maxSpread, spatialHi-spatialLo)
			frames[s.ID] = append(frames[s.ID], colors)
			previous = raw
		}
		metrics[s.ID] = map[string]any{"min_rgb": low, "max_rgb": high, "max_channel_step_per_100ms": maxStep, "max_simultaneous_red_spread": maxSpread, "frames": 600}
	}
	data, _ := json.Marshal(map[string]any{"fps": 10, "brightness": 65, "keys": keys, "scenes": scenes, "frames": frames})
	b, _ := json.MarshalIndent(metrics, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "metrics.json"), b, 0600); err != nil {
		panic(err)
	}
	// Read only the existing preview template; candidate frames are separate.
	template, err := os.ReadFile("cmd/preview/index.html")
	if err != nil {
		panic(err)
	}
	html := string(template)
	html = replace(html, "/*DATA*/", string(data))
	html = replace(html, "AULA RGB · 현재 효과", "AULA RGB · 대기 움직임 연구")
	html = replace(html, "키보드 전체가 함께 반응합니다", "낮은 샌드빛에 생동감을 더합니다")
	html = replace(html, "생명 파동 · 집중 효과 · 흐름 연결 · 밝기 65% · 10fps", "현재 효과와 연구안 비교 · 같은 샌드색 · 최대 밝기 유지 · 10fps · 실제 키보드 적용 전")
	html = replace(html, "F키도 현재 효과에 참여합니다. 작업별 F키 상태 표시는 없습니다. 이 미리보기는 USB 장치를 제어하지 않습니다.", "원본 RGB 신호로 표시합니다. 화면과 키보드의 실제 밝기는 다릅니다. 60초 끝에서는 처음으로 되돌아가므로 재생 루프 경계는 평가에서 제외하세요. USB 장치와 상주 앱은 변경하지 않습니다.")
	html = replace(html, "</h2><svg", "</h2><p>화면 보기 보정 <select id='gain'><option value='1'>원본 신호 1배</option><option value='3' selected>보기 보정 3배</option><option value='5'>보기 보정 5배</option></select> · 화면에서 어두운 움직임을 보기 위한 보정이며 키보드 설정과 무관합니다.</p><svg")
	html = replace(html, "r.setAttribute('stroke'", "r.style.filter='brightness('+Number($('gain').value)+')';r.setAttribute('stroke'")
	html = replace(html, "$('play').onclick=", "$('gain').onchange=()=>draw();$('play').onclick=")
	// Start at the recommended scene and run immediately; controls remain usable.
	html = replace(html, "let scene=D.scenes[0],n=20,playing=false", "let scene=D.scenes[1],n=20,playing=true")
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0600); err != nil {
		panic(err)
	}
	fmt.Println(filepath.Join(dir, "index.html"))
}
func replace(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}
