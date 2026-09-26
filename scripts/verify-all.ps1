# HomeAgent Automated Quality Gate Verification Script
# Sequence: Go Build -> Go Unit Tests -> TS Typecheck -> Frontend Unit Tests -> Playwright E2E

param (
    [switch]$SkipE2E = $false
)

$ErrorActionPreference = "Stop"

if ($env:SCOOP) {
    $goBin = "$env:SCOOP\apps\go\current\bin"
    if (Test-Path $goBin) {
        $env:Path = "$goBin;$env:PATH"
    }
}
$env:GOPROXY = 'https://goproxy.cn,direct'

$rootDir = Split-Path -Parent $PSScriptRoot
Set-Location $rootDir

$results = [ordered]@{}

Write-Host "===================================================="
Write-Host "  HomeAgent Quality Gate Verification Checklist     "
Write-Host "===================================================="

# 1. Go Build
Write-Host "`n[1/5] Checking Go backend build (go build)..."
try {
    go build -o homeagent.exe ./cmd/homeagent
    $results["Go Build"] = "PASSED"
    Write-Host "  -> Go build passed" -ForegroundColor Green
} catch {
    $results["Go Build"] = "FAILED"
    Write-Host "  -> Go build failed: $_" -ForegroundColor Red
    exit 1
}

# 2. Go Unit Tests
Write-Host "`n[2/5] Running Go unit tests (go test ./...)..."
try {
    go test ./... -count=1
    $results["Go Unit Tests"] = "PASSED"
    Write-Host "  -> Go unit tests passed" -ForegroundColor Green
} catch {
    $results["Go Unit Tests"] = "FAILED"
    Write-Host "  -> Go unit tests failed: $_" -ForegroundColor Red
}

# 3. Frontend Typecheck
Write-Host "`n[3/5] Running full workspace typecheck (pnpm -r run typecheck)..."
try {
    pnpm -r run typecheck
    $results["TS Typecheck"] = "PASSED"
    Write-Host "  -> TS typecheck 0 errors" -ForegroundColor Green
} catch {
    $results["TS Typecheck"] = "FAILED"
    Write-Host "  -> TS typecheck error: $_" -ForegroundColor Red
}

# 4. Frontend Unit Tests
Write-Host "`n[4/5] Running frontend unit tests (pnpm -r run test)..."
try {
    pnpm -r run test
    $results["Frontend Unit Tests"] = "PASSED"
    Write-Host "  -> Frontend unit tests passed" -ForegroundColor Green
} catch {
    $results["Frontend Unit Tests"] = "FAILED"
    Write-Host "  -> Frontend unit tests failed: $_" -ForegroundColor Red
}

# 5. Playwright E2E
if (-not $SkipE2E) {
    Write-Host "`n[5/5] Running Playwright E2E walkthrough..."
    try {
        pnpm --filter web exec playwright test e2e/mobile-android.spec.ts e2e/ux-walkthrough.spec.ts
        $results["Playwright E2E"] = "PASSED"
        Write-Host "  -> Playwright E2E passed" -ForegroundColor Green
    } catch {
        $results["Playwright E2E"] = "FAILED"
        Write-Host "  -> Playwright E2E failed: $_" -ForegroundColor Red
    }
} else {
    $results["Playwright E2E"] = "SKIPPED"
}

Write-Host "`n===================================================="
Write-Host "             Verification Summary                    "
Write-Host "===================================================="
foreach ($key in $results.Keys) {
    Write-Host ("{0,-25} : {1}" -f $key, $results[$key])
}
Write-Host "====================================================`n"

$hasFailure = $results.Values | Where-Object { $_ -eq "FAILED" }
if ($hasFailure) {
    exit 1
}
