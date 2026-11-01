@echo off
chcp 65001 >nul
title ChinaCPA 国产反代网关
cd /d "%~dp0"

:: Rebuild when source is newer than ccpa-server.exe (see needs-build.ps1).
powershell -NoProfile -ExecutionPolicy Bypass -File needs-build.ps1 ccpa-server.exe "cmd,internal" ".go"
if not errorlevel 1 goto :run

echo [ChinaCPA] 源码有更新，正在构建 ccpa-server.exe ...
go build -ldflags "-s -w" -o ccpa-server.exe ./cmd/chinacpa
if not errorlevel 1 goto :run
echo [ChinaCPA] 构建失败，仍尝试启动现有 exe。

:run
echo [ChinaCPA] 启动中... 面板 http://127.0.0.1:8318/
ccpa-server.exe -config config.json
pause
