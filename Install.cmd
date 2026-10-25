@echo off
setlocal
if not exist "%~dp0bin\aula-codex-rgb-setup.exe" (
  echo AULA setup is missing. Extract the entire ZIP before running this file.
  pause
  exit /b 1
)
start "" /wait "%~dp0bin\aula-codex-rgb-setup.exe" install
exit /b %errorlevel%