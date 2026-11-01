@echo off
chcp 65001 >nul
title ChinaCPA 浮动栏 (Pulse 质感)
cd /d "%~dp0"

:: Rebuild rail.exe when rail-rs source is newer than it.
powershell -NoProfile -ExecutionPolicy Bypass -File needs-build.ps1 rail.exe "rail-rs" ".rs,.toml"
if not errorlevel 1 goto :run

echo [Rail] 源码有更新，正在使用 Cargo 构建 rail.exe ...
cargo build --release --manifest-path rail-rs/Cargo.toml
if errorlevel 1 goto :build_failed
copy /y rail-rs\target\release\rail-rs.exe rail.exe >nul
goto :run

:build_failed
echo [Rail] 构建失败，中止启动。
pause
exit /b 1

:run
echo [Rail] 启动中... 贴屏幕右侧，鼠标悬停展开，右键菜单
rail.exe %*
