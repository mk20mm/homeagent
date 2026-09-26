# HomeAgent Contract-First Codegen Script
# Sequence: ent generate -> oapi-codegen -> openapi-typescript (web+admin)

$ErrorActionPreference = "Stop"

if ($env:SCOOP) {
    $goBin = "$env:SCOOP\apps\go\current\bin"
    if (Test-Path $goBin) {
        $env:Path = "$goBin;$env:PATH"
    }
}
$env:GOPROXY = 'https://goproxy.cn,direct'

$rootDir = Split-Path -Parent $PSScriptRoot

Write-Host "==> [1/3] Running Ent ORM codegen (internal/store/...)..."
Push-Location "$rootDir"
try {
    go generate ./internal/store/...
} finally {
    Pop-Location
}

Write-Host "==> [2/3] Running oapi-codegen for Go server stubs and types..."
Push-Location "$rootDir\internal\openapi"
try {
    go generate
} finally {
    Pop-Location
}

Write-Host "==> [3/3] Running openapi-typescript for web and admin TS schemas..."
Push-Location "$rootDir"
try {
    pnpm -r run gen-api
} finally {
    Pop-Location
}

Write-Host "==> Contract and code generation completed successfully."
