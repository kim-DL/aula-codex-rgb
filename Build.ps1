[CmdletBinding()]
param(
    [switch]$TestOnly,
    [switch]$Candidate,
    [string]$GoExe = 'go'
)
$ErrorActionPreference = 'Stop'
$goCommand = (Get-Command $GoExe -ErrorAction Stop).Source
$env:GOTOOLCHAIN = 'local'
function Invoke-Go {
    & $goCommand @args
    if ($LASTEXITCODE -ne 0) { throw ('Go command failed: ' + ($args -join ' ')) }
}
Push-Location (Join-Path $PSScriptRoot 'upstream')
try {
    Invoke-Go version
    # Mock protocol tests only; never open the keyboard while the app owns it.
    Invoke-Go test ./pkg/aula -run '^TestRealtime' -skip '^TestRealtimeHardware' -count=1
} finally { Pop-Location }
Push-Location (Join-Path $PSScriptRoot 'app')
try {
    Invoke-Go test ./...
    Invoke-Go test -tags rgb_gui ./...
    if (-not $TestOnly) {
        $output = Join-Path $PSScriptRoot 'bin'
        [void][IO.Directory]::CreateDirectory($output)
        $suffix = if ($Candidate) { '-candidate' } else { '' }
        Invoke-Go build -buildvcs=false -trimpath -ldflags '-s -w' -o (Join-Path $output ('aula-codex-rgb' + $suffix + '.exe')) .
        Invoke-Go build -buildvcs=false -trimpath -tags rgb_gui -ldflags '-s -w -H=windowsgui' -o (Join-Path $output ('aula-codex-rgb-background' + $suffix + '.exe')) .
        Invoke-Go build -buildvcs=false -trimpath -ldflags '-s -w -H=windowsgui' -o (Join-Path $output ('aula-codex-rgb-setup' + $suffix + '.exe')) ./cmd/desktop-setup
    }
} finally { Pop-Location }
