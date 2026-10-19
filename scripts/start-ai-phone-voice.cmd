@echo off
setlocal
cd /d "%~dp0.."
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0start-ai-phone-voice.ps1" %*
if errorlevel 1 (
  echo.
  echo 启动失败。按任意键关闭窗口。
  pause >nul
)
endlocal
