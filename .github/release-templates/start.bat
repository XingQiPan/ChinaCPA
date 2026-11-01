@echo off
rem ChinaCPA 启动脚本（发布包内附）
chcp 65001 >nul
title ChinaCPA 国产反代网关
cd /d "%~dp0"

if not exist config.json (
  echo [ChinaCPA] 首次启动，将生成 config.json
  echo [ChinaCPA] 请启动后在面板或 config.json 中修改 api_key 与 panel_password
)

chinacpa.exe -config config.json
pause
