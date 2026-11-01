#!/bin/sh
# ChinaCPA 启动脚本（发布包内附）
set -e
cd "$(dirname "$0")" || exit 1

if [ ! -f config.json ]; then
  echo "[ChinaCPA] 首次启动，将生成 config.json"
  echo "[ChinaCPA] 请启动后在面板或 config.json 中修改 api_key 与 panel_password"
fi

exec ./chinacpa -config config.json
