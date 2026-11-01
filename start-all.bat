@echo off
chcp 65001 >nul
title ChinaCPA 一键启动 (网关 + Pulse 侧边栏)
cd /d "%~dp0"

:: Rebuild gateway when source is newer than ccpa-server.exe.
powershell -NoProfile -ExecutionPolicy Bypass -File needs-build.ps1 ccpa-server.exe "cmd,internal" ".go"
if not errorlevel 1 goto :gateway_ok

echo [1/3] 源码有更新，正在构建 ccpa-server.exe ...
go build -ldflags "-s -w" -o ccpa-server.exe ./cmd/chinacpa
if errorlevel 1 goto :gateway_build_failed

:gateway_ok
echo [2/3] 正在启动 ChinaCPA 后端网关...
start "ChinaCPA Gateway" ccpa-server.exe -config config.json

:: Rebuild rail.exe when rail-rs source is newer than it.
powershell -NoProfile -ExecutionPolicy Bypass -File needs-build.ps1 rail.exe "rail-rs" ".rs,.toml"
if not errorlevel 1 goto :rail_ok

echo [3/3] 正在构建 rail.exe (cargo release) ...
cargo build --release --manifest-path rail-rs/Cargo.toml
if errorlevel 1 goto :rail_build_failed
copy /y rail-rs\target\release\rail-rs.exe rail.exe >nul

:rail_ok
echo [3/3] 正在启动 Pulse 质感悬浮桌面挂件 (rail.exe)...
start "" "%~dp0rail.exe"
goto :done

:rail_build_failed
echo [Rail] 构建失败，跳过浮动栏。
goto :done

:gateway_build_failed
echo [ChinaCPA] 构建失败，中止启动。
pause
exit /b 1

:done
echo.
echo ========================================================
echo ChinaCPA 网关与 Pulse 风格侧边栏已同时启动
echo 管理面板: http://127.0.0.1:8318/
echo 接口地址: http://127.0.0.1:8318/v1
echo 桌面挂件: 已停靠在主屏幕最右侧
echo 鼠标移至右边缘展开，鼠标移出自动收起为细条
echo 鼠标悬停圆环滑出详情卡片，右键打开快捷菜单
echo ========================================================
timeout /t 3 >nul
