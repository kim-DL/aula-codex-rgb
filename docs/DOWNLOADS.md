# 실행 파일 다운로드와 더블클릭 설치

[최신 GitHub Release](https://github.com/kim-DL/aula-codex-rgb/releases/latest)에서 `aula-codex-rgb-v0.1.1-windows-amd64.zip`을 받습니다. Windows x64용이며 Go 설치나 PowerShell 명령 입력은 필요하지 않습니다.

## 처음 설치

1. ZIP **전체**를 계속 사용할 폴더에 압축 해제합니다. ZIP 내부에서 실행하지 마세요.
2. 압축 해제한 폴더의 **Install.cmd**를 탐색기에서 더블클릭합니다.
3. 한국어 설치 확인 창에서 **예**를 누릅니다. 현재 사용자 로그인 자동 실행과 AULA 훅을 등록한 뒤 앱 실행을 요청합니다.
4. Codex 훅 기능이 활성화되어 있어야 합니다. 제품이 훅 검토·신뢰 등록을 요구하면 추가된 AULA 명령을 확인한 뒤 정상 절차로 등록합니다. 설치 도구는 신뢰 설정이나 승인 정책을 우회하지 않습니다.
5. 작업 표시줄의 숨겨진 아이콘(`^`)에서 AULA RGB 트레이를 확인하고 로컬 Codex·Work 작업을 보내 반응을 확인합니다.

설치 후 폴더를 옮기지 마세요. 다른 위치의 앱이 자동 실행에 등록되어 있으면 기존 위치에서 먼저 제거하도록 안내하며 덮어쓰지 않습니다. 탐색기에서 실행하면 Codex 패키지 내부 셸의 경로·레지스트리 가상화를 피할 수 있습니다.

## 파일별 용도

| 파일 | 역할 |
| --- | --- |
| **Install.cmd** | 현재 사용자용 설치와 앱 실행 |
| **Run.cmd** | 설치 등록을 바꾸지 않고 앱 실행 |
| **Uninstall.cmd** | 확인 후 앱 정상 종료를 기다리고 자동 실행·이 설치 위치의 훅 제거 |
| `bin/aula-codex-rgb-background.exe` | 실제 백그라운드·트레이 앱 |
| `bin/aula-codex-rgb.exe` | 고급 사용자용 명령 도구 |
| `bin/aula-codex-rgb-setup.exe` | CMD에서 호출하는 한국어 안내 프로그램 |

CMD를 실행하면 창이 잠깐 보일 수 있습니다. 파일이 빠졌다면 ZIP 전체를 다시 확인하세요. 실행 파일은 디지털 서명이 없는 개인 프로젝트 배포본으로, Windows 또는 조직 정책에 따라 추가 확인이나 실행 제한이 있을 수 있습니다. 관리자 권한을 요청하거나 PowerShell 실행 정책을 바꾸지 않습니다.

## 종료·제거·업데이트

- **현재 실행만 종료**: 트레이의 **종료**를 사용합니다. 다음 로그인 자동 실행은 유지됩니다.
- **자동 실행과 연동 제거**: 기존 폴더의 **Uninstall.cmd**를 더블클릭하고 확인합니다. 앱 종료가 확인되지 않으면 연동을 제거하지 않고 오류를 보여 줍니다. 앱 폴더·사용자 설정과 다른 Codex 훅은 보존합니다. 제거 완료 안내 뒤 필요하면 앱 폴더를 직접 정리하세요.
- **같은 위치에서 업데이트**: 앱을 정상 종료하고 기존 파일·설정을 보관한 뒤 새 ZIP의 실행 파일과 CMD 파일을 함께 교체하고 **Run.cmd**로 실행합니다.
- **위치를 바꾸기**: 이전 위치에서 제거한 뒤 새 위치의 **Install.cmd**를 실행합니다. v0.1.0은 이전 폴더의 CLI로 `stop` → `uninstall`을 먼저 진행해야 합니다.

## 고급 사용자용 명령

명령을 사용하려는 경우에만 일반 Windows PowerShell에서 압축 해제 폴더로 이동합니다.

```powershell
.\bin\aula-codex-rgb.exe install
.\bin\aula-codex-rgb.exe run
.\bin\aula-codex-rgb.exe status
.\bin\aula-codex-rgb.exe lcd-status
.\bin\aula-codex-rgb.exe pause
.\bin\aula-codex-rgb.exe resume
.\bin\aula-codex-rgb.exe stop
.\bin\aula-codex-rgb.exe uninstall
```

릴리스의 별도 `SHA256SUMS.txt`와 비교해 ZIP 해시를 확인할 수 있습니다. ZIP 내부의 같은 이름 파일은 내부 파일 검증용입니다.

```powershell
Get-FileHash .\aula-codex-rgb-v0.1.1-windows-amd64.zip -Algorithm SHA256
```

## 지원 범위와 기본 동작

AULA F108 Pro USB-C 유선 `0C45:800A`와 로컬 Codex·Work를 대상으로 확인한 앱입니다. 일반 ChatGPT 모드·다른 키보드·모든 펌웨어 또는 제품 업데이트 조합의 호환성을 보장하지 않습니다.

밝기 65%·10fps, 파랑 배경의 짧은 흰색 코드 스캔, 문서 읽기·수정 흰색 파동, 파랑+흰색 생각 중 생명 파동을 유지합니다. LCD는 작업 중 경과시간, 최종 응답 뒤 PC 날짜·시각으로 돌아옵니다. 최근 결과는 RAM 한 건이고 시간 이력을 저장하지 않습니다.

앱은 단일 USB 전송기를 유지하며 드라이버 교체나 관리자 서비스 설치를 하지 않습니다. 이 배포본에는 사용자 설정·개인 훅 사본·인증 정보·대화 로그·실기 캡처·PC별 복원 백업을 넣지 않습니다. 검증 범위는 [VALIDATION.md](VALIDATION.md)를 참고하세요.
