param(
  [int]$ApiPort = 8080,
  [int]$WebPort = 5173
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

Write-Host "Stopping existing listeners"
foreach ($port in @($ApiPort, $WebPort)) {
  $connections = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
  foreach ($connection in $connections) {
    Stop-Process -Id $connection.OwningProcess -Force
  }
}

Write-Host "Starting Go API on port $ApiPort"
Start-Process -WindowStyle Hidden -FilePath "powershell" -ArgumentList @(
  "-NoProfile",
  "-Command",
  "`$env:PORT='$ApiPort'; cd '$Root\backend'; go run ./cmd/server"
)

Write-Host "Starting Vite web on port $WebPort"
Start-Process -WindowStyle Hidden -FilePath "powershell" -ArgumentList @(
  "-NoProfile",
  "-Command",
  "cd '$Root'; npm run dev -- --host 127.0.0.1 --port $WebPort"
)

Start-Sleep -Seconds 3
Invoke-RestMethod "http://localhost:$ApiPort/api/health" | Out-Null
Write-Host "Local deployment ready"
Write-Host "API: http://localhost:$ApiPort"
Write-Host "Web: http://localhost:$WebPort"
