# Copyright (c) Kusari <https://www.kusari.dev/>
# SPDX-License-Identifier: MIT
#
# Run the Windows checks locally, mirroring the "windows-e2e" job in
# .github/workflows/pr.yaml. That workflow remains the source of truth; this
# script exists so the same ground can be covered on a real machine without
# waiting on a push.
#
#   PS> .\scripts\windows-local-test.ps1
#   PS> .\scripts\windows-local-test.ps1 -SkipSbom     # no Waybill download
#
# Requires: Go (version from go.mod) and Git for Windows on PATH.
# Nothing here needs Kusari credentials.

[CmdletBinding()]
param(
    # Skip "platform generate", which downloads ~23 MB of Waybill on first run.
    [switch]$SkipSbom
)

# Native commands write progress to stderr; PowerShell can turn that into
# terminating errors. Exit codes are checked explicitly instead.
$ErrorActionPreference = "Continue"

$script:Failures = @()

function Step {
    param([string]$Name, [scriptblock]$Body)
    Write-Host ""
    Write-Host "=== $Name ===" -ForegroundColor Cyan
    try {
        & $Body
        Write-Host "PASS: $Name" -ForegroundColor Green
    } catch {
        Write-Host "FAIL: $Name -- $_" -ForegroundColor Red
        $script:Failures += $Name
    }
}

Write-Host "Windows local test run" -ForegroundColor Cyan
Get-ComputerInfo | Select-Object WindowsProductName, OsVersion | Format-List
go version
git --version

# Developer Mode decides whether the symlink tests run or skip. Worth knowing
# up front, because those cover the part of the archiver most likely to differ
# on Windows.
$devMode = Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock" `
    -Name AllowDevelopmentWithoutDevLicense -ErrorAction SilentlyContinue
if ($devMode.AllowDevelopmentWithoutDevLicense -eq 1) {
    Write-Host "Developer Mode: ON (symlink tests will run)" -ForegroundColor Green
} else {
    Write-Host "Developer Mode: OFF -- symlink tests will SKIP." -ForegroundColor Yellow
    Write-Host "  Enable at Settings > System > For developers to cover them." -ForegroundColor Yellow
}

# core.autocrlf=true rewrites tracked files to CRLF on checkout. .gitattributes
# pins eol=lf so this should already be inert; a clone made before that landed
# can still be holding CRLF, which breaks the fixture-hashing tests.
$crlf = git ls-files --eol -- pkg/configuration/testdata 2>$null | Where-Object { $_ -match "w/crlf" }
if ($crlf) {
    Write-Host "WARNING: testdata has CRLF in the working tree. Run:" -ForegroundColor Yellow
    Write-Host "  git rm --cached -r . ; git reset --hard" -ForegroundColor Yellow
}

Step "go build" {
    go build -v ./...
    if ($LASTEXITCODE -ne 0) { throw "go build exited $LASTEXITCODE" }
}

Step "go vet" {
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet exited $LASTEXITCODE" }
}

Step "go test" {
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test exited $LASTEXITCODE" }
}

Step "build kusari.exe" {
    go build -o kusari.exe ./kusari
    if (-not (Test-Path kusari.exe)) { throw "kusari.exe was not produced" }
}

Step "CLI starts" {
    .\kusari.exe --version
    if ($LASTEXITCODE -ne 0) { throw "--version exited $LASTEXITCODE" }
}

Step "scan bundle from a path containing spaces" {
    $repo = Join-Path $env:TEMP "kusari local test repo"
    if (Test-Path $repo) { Remove-Item -Recurse -Force $repo }
    New-Item -ItemType Directory -Force -Path $repo | Out-Null
    Push-Location $repo
    try {
        git init --initial-branch=main 2>&1 | Out-Null
        git config user.email "local@example.com"
        git config user.name "Local"
        git config core.autocrlf true   # the Git for Windows default
        New-Item -ItemType Directory -Force -Path "sub dir" | Out-Null
        Set-Content -Path "main.go" -Value "package main`n`nfunc main() {}"
        Set-Content -Path "sub dir\lib.go" -Value "package lib"
        git add -A
        git commit -q -m "initial"
        Set-Content -Path "main.go" -Value "package main`n`nfunc main() { println(1) }"
    } finally {
        Pop-Location
    }

    # Redirected via Start-Process: merging stderr with 2>&1 makes PowerShell
    # render a red NativeCommandError block even on success.
    $out = Join-Path $env:TEMP "kusari-scan.out"
    $errFile = Join-Path $env:TEMP "kusari-scan.err"
    # -ArgumentList joins with spaces and quotes nothing, so the path needs its own quotes.
    $proc = Start-Process -FilePath ".\kusari.exe" `
        -ArgumentList @("repo", "scan", ('"' + $repo + '"'), "HEAD") `
        -NoNewWindow -Wait -PassThru `
        -RedirectStandardOutput $out -RedirectStandardError $errFile
    $text = (Get-Content $out -Raw) + (Get-Content $errFile -Raw)
    Write-Host $text

    if ($text -match "accepts \d+ arg") { throw "the path argument was split -- quoting problem in this script" }
    foreach ($bad in @("failed to package directory",
                       "Cannot create a file when that file already exists",
                       "error taring source code", "error compressing file",
                       "'sh' is not recognized", "'tar' is not recognized",
                       "'bzip2' is not recognized")) {
        if ($text -match [regex]::Escape($bad)) { throw "packaging pipeline failed: $bad" }
    }
    # Without credentials the scan can only get as far as the auth gate, which
    # is exactly the point: everything before it is the local pipeline.
    if ($text -notmatch "auth" -and $text -notmatch "token" -and $text -notmatch "login") {
        throw "expected the scan to stop at authentication, got: $text"
    }
}

Step "refused connections are diagnosed" {
    $out = Join-Path $env:TEMP "kusari-refused.out"
    $errFile = Join-Path $env:TEMP "kusari-refused.err"
    $proc = Start-Process -FilePath ".\kusari.exe" `
        -ArgumentList @("connectivity", "check", "--timeout", "10s",
                        "--platform-url", "http://127.0.0.1:1") `
        -NoNewWindow -Wait -PassThru `
        -RedirectStandardOutput $out -RedirectStandardError $errFile
    $text = (Get-Content $out -Raw) + (Get-Content $errFile -Raw)
    # A failing probe exits 1; that is expected. Only the wording matters --
    # it proves the Winsock error was recognised rather than falling through
    # to the generic branch.
    if ($text -notmatch "actively refused the connection") {
        throw "a refused connection was not diagnosed as one: $text"
    }
}

Step "MCP client config paths resolve" {
    .\kusari.exe ai list
    if ($LASTEXITCODE -ne 0) { throw "ai list exited $LASTEXITCODE" }
}

if (-not $SkipSbom) {
    Step "generate an SBOM (downloads Waybill on first run)" {
        .\kusari.exe platform generate -- --path .
        if ($LASTEXITCODE -ne 0) { throw "platform generate exited $LASTEXITCODE" }
        if (-not (Test-Path project.cdx.json)) { throw "no SBOM written" }
        $sbom = Get-Content project.cdx.json -Raw | ConvertFrom-Json
        Write-Host "SBOM format=$($sbom.bomFormat) components=$($sbom.components.Count)"
        if ($sbom.bomFormat -ne "CycloneDX") { throw "unexpected bomFormat" }
        if ($sbom.components.Count -lt 1) { throw "SBOM has no components" }

        $bin = Get-ChildItem -Path (Join-Path $env:USERPROFILE ".kusari\bin") `
            -Filter "waybill-*.exe" -ErrorAction SilentlyContinue
        if (-not $bin) { throw "no waybill-*.exe in ~/.kusari/bin" }
        $bin | Select-Object Name, Length | Format-Table
    }
}

Write-Host ""
if ($script:Failures.Count -eq 0) {
    Write-Host "All checks passed." -ForegroundColor Green
    exit 0
}
Write-Host "Failed: $($script:Failures -join ', ')" -ForegroundColor Red
exit 1
