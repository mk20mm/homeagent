# HomeAgent Harness Environment & Process Hygiene Script
# Resolves: Go environment paths, leftover port 8080 processes, proxy detection

$ErrorActionPreference = "Stop"

Write-Host "==> [1/3] Configuring Go and Package Manager environment..."
if ($env:SCOOP) {
    $goBin = "$env:SCOOP\apps\go\current\bin"
    if (Test-Path $goBin) {
        $env:Path = "$goBin;$env:PATH"
    }
}
$env:GOPROXY = 'https://goproxy.cn,direct'

Write-Host "==> [2/3] Cleaning leftover homeagent.exe processes..."
Get-Process -Name "homeagent", "homeagent.exe" -ErrorAction SilentlyContinue | ForEach-Object {
    Write-Host "    Terminating PID: $($_.Id)"
    Stop-Process -Id $_.Id -Force
}

# Check port 8080
$portOccupied = Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
if ($portOccupied) {
    Write-Host "    Warning: Port 8080 occupied by PID $($portOccupied.OwningProcess), terminating..."
    Stop-Process -Id $portOccupied.OwningProcess -Force -ErrorAction SilentlyContinue
}

Write-Host "==> [3/3] Checking proxy environment..."
if ($env:HTTP_PROXY -or $env:http_proxy) {
    Write-Host "    Notice: Local proxy detected. Append --noproxy '*' to curl for localhost requests."
}

Write-Host "==> Environment ready."
