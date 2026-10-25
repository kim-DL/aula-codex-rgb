//go:build windows

// A small native dialog wrapper around the existing per-user installer.
// It never opens HID, elevates privileges, or changes hook trust.
package main

import (
	"aula-codex-rgb-lab/internal/instance"
	"aula-codex-rgb-lab/internal/setup"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type operations struct {
	registration                    func() (string, error)
	install, uninstall, start, stop func() error
	running                         func() (bool, error)
	waitStopped                     func() error
}

func perform(action, background string, op operations) error {
	if action != "install" && action != "run" && action != "uninstall" {
		return fmt.Errorf("배포 폴더의 Install.cmd / Run.cmd / Uninstall.cmd를 사용해 주세요")
	}
	registered, err := op.registration()
	if err != nil {
		return fmt.Errorf("자동 실행 등록을 확인하지 못했습니다: %w", err)
	}
	if registered != "" && !strings.EqualFold(strings.TrimSpace(registered), `"`+background+`" run --background`) {
		return fmt.Errorf("다른 위치의 AULA RGB가 등록되어 있습니다. 기존 위치에서 제거한 뒤 다시 진행해 주세요.\n\n등록된 명령: %s", registered)
	}
	switch action {
	case "install":
		if err := op.install(); err != nil {
			return err
		}
		if err := op.start(); err != nil {
			return fmt.Errorf("연동 등록은 완료했지만 앱 실행에 실패했습니다. Run.cmd로 다시 실행해 주세요: %w", err)
		}
	case "run":
		return op.start()
	case "uninstall":
		running, err := op.running()
		if err != nil {
			return err
		}
		if running {
			if err := op.stop(); err != nil {
				return fmt.Errorf("앱 종료 요청에 실패하여 연동을 제거하지 않았습니다: %w", err)
			}
			if err := op.waitStopped(); err != nil {
				return err
			}
		}
		return op.uninstall()
	}
	return nil
}

func registration() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
	if err == registry.ErrNotExist {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	defer k.Close()
	value, _, err := k.GetStringValue("AulaCodexRgb")
	if err == registry.ErrNotExist {
		return "", nil
	}
	return value, err
}

func cliCommand(cli, action string) error {
	cmd := exec.Command(cli, action)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %s (%w)", action, strings.TrimSpace(string(output)), err)
	}
	return nil
}

func waitStopped() error {
	for i := 0; i < 40; i++ {
		running, err := instance.IsRunning()
		if err != nil {
			return err
		}
		if !running {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("앱이 아직 종료되지 않아 연동을 제거하지 않았습니다. 트레이에서 종료한 뒤 다시 시도해 주세요")
}

var messageBox = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")

func dialog(text string, flags uintptr) uintptr {
	title, _ := windows.UTF16PtrFromString("AULA Codex RGB")
	message, _ := windows.UTF16PtrFromString(text)
	result, _, _ := messageBox.Call(0, uintptr(unsafe.Pointer(message)), uintptr(unsafe.Pointer(title)), flags|0x10000)
	return result
}

func main() {
	if len(os.Args) != 2 {
		dialog("배포 폴더의 Install.cmd(설치), Run.cmd(실행), Uninstall.cmd(제거)를 더블클릭해 주세요.", 0x40)
		return
	}
	action := os.Args[1]
	if action != "install" && action != "run" && action != "uninstall" {
		dialog("지원하지 않는 작업입니다.", 0x10)
		os.Exit(1)
	}
	exe, err := os.Executable()
	if err != nil {
		dialog(err.Error(), 0x10)
		os.Exit(1)
	}
	bin := filepath.Dir(exe)
	cli, background := filepath.Join(bin, "aula-codex-rgb.exe"), filepath.Join(bin, "aula-codex-rgb-background.exe")
	for _, path := range []string{cli, background} {
		info, e := os.Stat(path)
		if e != nil || info.IsDir() {
			dialog("실행 파일을 찾을 수 없습니다. ZIP 전체를 압축 해제해 주세요.\n\n"+path, 0x10)
			os.Exit(1)
		}
	}
	if action == "install" && dialog("현재 폴더에 AULA Codex RGB를 설치하고 실행할까요?\n\n현재 사용자 로그인 자동 실행과 Codex 훅을 등록합니다. 기존 훅은 백업 후 보존합니다.\n설치 후 이 폴더를 옮기지 마세요. 관리자 권한은 필요하지 않습니다.", 0x24) != 6 {
		return
	}
	if action == "uninstall" && dialog("AULA Codex RGB를 종료하고 자동 실행·훅 연동을 제거할까요?\n\n다른 Codex 훅과 앱 폴더·사용자 설정은 유지합니다. 파일 삭제와 재부팅은 하지 않습니다.", 0x124) != 6 {
		return
	}
	op := operations{
		registration: registration,
		install:      func() error { return setup.Install(cli) },
		uninstall:    func() error { return setup.Uninstall(cli) },
		start:        func() error { return cliCommand(cli, "run") },
		stop:         func() error { return cliCommand(cli, "stop") },
		running:      instance.IsRunning, waitStopped: waitStopped,
	}
	if err := perform(action, background, op); err != nil {
		dialog("처리하지 못했습니다.\n\n"+err.Error(), 0x10)
		os.Exit(1)
	}
	if action == "install" {
		dialog("설치·실행 요청을 마쳤습니다. 작업 표시줄의 AULA RGB 트레이에서 상태를 확인해 주세요.\n\nCodex 훅 기능이 활성화되어 있어야 합니다. 제품이 훅 검토·신뢰 등록을 요구하면 AULA 명령을 확인한 뒤 정상 절차로 등록해 주세요.\n\n다음 Windows 로그인부터 자동 실행됩니다. RGB 65%·10fps와 LCD 동작은 그대로입니다.", 0x40)
	}
	if action == "uninstall" {
		dialog("앱 종료와 자동 실행·훅 연동 제거를 마쳤습니다.\n\n원하면 이제 이 앱 폴더를 직접 삭제할 수 있습니다. 사용자 설정과 다른 Codex 훅은 보존했습니다.", 0x40)
	}
}
