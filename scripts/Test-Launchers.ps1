[CmdletBinding()]
param([string]$GoExe = 'go')
$ErrorActionPreference = 'Stop'
$workspace = Split-Path $PSScriptRoot -Parent
$fixture = Join-Path $workspace ('bin\launcher-tests\한글 공백 & path ' + [Guid]::NewGuid().ToString('N'))
[void][IO.Directory]::CreateDirectory((Join-Path $fixture 'bin'))
# The real CMD launchers receive a harmless recording EXE. No real install/HID.
$source = @'
package main
import ("encoding/json"; "os")
func main() {
    b, _ := json.Marshal(os.Args)
    if err := os.WriteFile(os.Getenv("AULA_LAUNCHER_TEST_LOG"), b, 0600); err != nil { os.Exit(8) }
    if os.Getenv("AULA_LAUNCHER_TEST_FAIL") == "1" { os.Exit(7) }
}
'@
$sourcePath = Join-Path $fixture 'fixture.go'
[IO.File]::WriteAllText($sourcePath, $source, [Text.UTF8Encoding]::new($false))
$stub = Join-Path $fixture 'bin\aula-codex-rgb-setup.exe'
& $GoExe build -trimpath -ldflags '-s -w -H=windowsgui' -o $stub $sourcePath
if ($LASTEXITCODE -ne 0) { throw 'Fixture build failed' }
$previousLog = $env:AULA_LAUNCHER_TEST_LOG
$previousFail = $env:AULA_LAUNCHER_TEST_FAIL
try {
    $env:AULA_LAUNCHER_TEST_LOG = Join-Path $fixture 'arguments.json'
    foreach ($entry in @(@('Install.cmd','install'), @('Run.cmd','run'), @('Uninstall.cmd','uninstall'))) {
        $launcher = Join-Path $fixture $entry[0]
        Copy-Item -LiteralPath (Join-Path $workspace $entry[0]) -Destination $launcher
        foreach ($fail in @('0','1')) {
            $env:AULA_LAUNCHER_TEST_FAIL = $fail
            $process = [Diagnostics.Process]::new()
            $process.StartInfo.FileName = $env:ComSpec
            $process.StartInfo.Arguments = '/d /s /c ""' + $launcher + '""'
            $process.StartInfo.UseShellExecute = $false
            [void]$process.Start()
            $process.WaitForExit()
            $code = $process.ExitCode
            $process.Dispose()
            $expected = if ($fail -eq '1') { 7 } else { 0 }
            if ($code -ne $expected) { throw ($entry[0] + ': wrong exit code ' + $code) }
            $actual = Get-Content -LiteralPath $env:AULA_LAUNCHER_TEST_LOG -Raw | ConvertFrom-Json
            if ($actual.Count -ne 2 -or $actual[0] -ne $stub -or $actual[1] -ne $entry[1]) {
                throw ($entry[0] + ': wrong executable or action')
            }
        }
    }
    'PASS: 3 CMD launchers, Unicode/spaces/ampersand path, success/error propagation (6 cases). No real installation or USB access.'
} finally {
    $env:AULA_LAUNCHER_TEST_LOG = $previousLog
    $env:AULA_LAUNCHER_TEST_FAIL = $previousFail
}
