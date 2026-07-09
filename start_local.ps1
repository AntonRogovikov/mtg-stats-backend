# Локальный запуск бэкенда mtg_stats (Windows).
# Загружает переменные из .env (Go сам их не читает) и запускает сервер.
# Требуется: локальный PostgreSQL (служба postgresql-x64-18) и Go.
Set-Location $PSScriptRoot

if (-not (Test-Path .env)) {
    Write-Error ".env не найден рядом со скриптом"
    exit 1
}

Get-Content .env | ForEach-Object {
    if ($_ -match '^\s*([A-Za-z_]+)\s*=\s*(.+)$') {
        [System.Environment]::SetEnvironmentVariable($Matches[1], $Matches[2].Trim())
    }
}

Write-Host "Запуск бэкенда на порту $env:PORT (GIN_MODE=$env:GIN_MODE)..."
go run .
