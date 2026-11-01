# needs-build.ps1 — 判断目标 exe 是否落后于源码，需重新构建时 exit 1。
# 用法: powershell -NoProfile -ExecutionPolicy Bypass -File needs-build.ps1 <exe> <源码目录> <扩展名,逗号分隔>
# 例:   powershell ... -File needs-build.ps1 ccpa-server.exe "cmd,internal" ".go"
param(
  [Parameter(Mandatory=$true)][string]$Exe,
  [Parameter(Mandatory=$true)][string]$SourceDir,
  [Parameter(Mandatory=$true)][string]$Extensions
)

if (-not (Test-Path $Exe)) { exit 1 }

$exeTime = (Get-Item $Exe).LastWriteTime
$exts = $Extensions.Split(',') | ForEach-Object { $_.Trim() }

$dirs = $SourceDir.Split(',') | Where-Object { Test-Path $_ }
if (-not $dirs) { exit 0 }

$newer = Get-ChildItem -Recurse -File -Path $dirs -Include $exts -ErrorAction SilentlyContinue |
  Where-Object { $_.FullName -notmatch '\\target\\' -and $_.LastWriteTime -gt $exeTime } |
  Select-Object -First 1

if ($newer) { exit 1 } else { exit 0 }
