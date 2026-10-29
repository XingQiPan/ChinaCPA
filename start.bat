@echo off
chcp 65001 >nul
title ChinaCPA 国产反代网关
cd /d "%~dp0"
if not exist ccpa-server.exe (
  echo [ChinaCPA] 未找到 ccpa-server.exe，正在构建...
  go build -ldflags "-s -w" -o ccpa-server.exe ./cmd/chinacpa
)
echo [ChinaCPA] 启动中... 面板 http://127.0.0.1:8318/
ccpa-server.exe -config config.json
pause
