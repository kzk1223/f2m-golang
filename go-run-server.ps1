$ErrorActionPreference = "Stop"

# ---------------------------------------------
# 設定
# ---------------------------------------------
$ProjectRoot = "C:\inetpub\wwwroot\@golang\f2m-golang"
$ServerMain  = Join-Path $ProjectRoot "cmd\server\main.go"

# ---------------------------------------------
# 存在確認
# ---------------------------------------------
if (!(Test-Path $ServerMain)) {
    Write-Host "server main file not found: $ServerMain" -ForegroundColor Red
    exit 1
}

# ---------------------------------------------
# Goサーバー起動
# ---------------------------------------------
Push-Location $ProjectRoot

try {
    Write-Host "start f2m-golang server..." -ForegroundColor Green
    Write-Host "http://127.0.0.1:8088/"
    Write-Host "http://f2m-golang.local:8081/"
    Write-Host ""

    go run .\cmd\server
}
finally {
    Pop-Location
}
