# 실행 파일 다운로드

[최신 GitHub Release](https://github.com/kim-DL/aula-codex-rgb/releases/latest)에서 `aula-codex-rgb-v0.1.0-windows-amd64.zip`과 `SHA256SUMS.txt`를 받습니다. Windows x64용이며 사용 시 Go 설치는 필요하지 않습니다.

## 압축 해제와 확인

1. ZIP 전체를 앞으로 계속 사용할 폴더에 압축 해제합니다. ZIP 내부에서 직접 실행하지 마세요.
2. `SHA256SUMS.txt`와 비교해 내려받은 ZIP을 확인할 수 있습니다.

```powershell
Get-FileHash .\aula-codex-rgb-v0.1.0-windows-amd64.zip -Algorithm SHA256
```

패키지에는 `bin/aula-codex-rgb-background.exe`, `bin/aula-codex-rgb.exe`, 안내·버전 명세·서드파티 라이선스가 들어 있습니다. 실행 파일은 서명되지 않은 개인 프로젝트 배포본입니다. Windows나 조직 정책에 따라 추가 확인 또는 실행 제한이 있을 수 있습니다.

## 실행과 연동

Windows 시작 메뉴에서 연 **일반 PowerShell**로 압축 해제 폴더에 이동합니다.

```powershell
.\bin\aula-codex-rgb.exe install
.\bin\aula-codex-rgb.exe run
```

현재 사용자 로그인 자동 실행과 AULA 훅을 등록합니다. 훅이 제품의 검토·신뢰 등록을 요구하면 정상 절차로 확인해야 합니다. 다른 훅과 승인 정책은 유지합니다. 등록된 명령은 파일 경로를 이용하므로 설치한 폴더를 이후 옮기지 않는 편이 좋습니다.

Codex 내부 셸에서의 설치는 AppData/레지스트리 가상화로 실제 로그인 환경과 달라질 수 있습니다. 이미 다른 위치에서 사용 중인 앱을 업그레이드한다면 기존 앱을 정상 종료하고 기존 위치의 파일을 보관한 뒤 두 실행 파일을 함께 교체합니다. 설치 위치를 옮겼다면 새 위치에서 연동 등록을 다시 진행해야 합니다.

```powershell
.\bin\aula-codex-rgb.exe status
.\bin\aula-codex-rgb.exe lcd-status
.\bin\aula-codex-rgb.exe pause
.\bin\aula-codex-rgb.exe resume
.\bin\aula-codex-rgb.exe stop
.\bin\aula-codex-rgb.exe uninstall
```

트레이의 종료 메뉴로도 닫을 수 있습니다. 앱은 중복 USB 소유자를 차단하며, 드라이버 교체나 관리자 서비스 설치를 하지 않습니다.

## 지원 범위

AULA F108 Pro USB-C 유선 `0C45:800A`와 로컬 Codex·Work를 대상으로 확인한 앱입니다. 일반 ChatGPT 모드·다른 키보드·모든 펌웨어 또는 제품 업데이트 조합의 호환성을 보장하지 않습니다.

기본은 밝기 65%·10fps입니다. 코드 작업은 파란 배경의 짧은 흰색 스캔, 문서 읽기·수정은 흰색 문서 파동, 생각 중은 파랑+흰색 생명 파동입니다. LCD는 작업 중 경과시간, 최종 응답 뒤 PC 날짜·시각으로 돌아옵니다. 최근 결과는 RAM 한 건이고 시간 이력을 저장하지 않습니다.

이 배포본에는 사용자 설정·개인 훅 사본·인증 정보·대화 로그·실기 캡처·PC별 복원 백업을 넣지 않습니다. 검증 범위는 [VALIDATION.md](VALIDATION.md)를 참고하세요.
