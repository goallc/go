$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/download-windows-dependency.ps1"

# Exercise the cache with real files but no network or Windows installers.
$root = Join-Path ([IO.Path]::GetTempPath()) ("goallc download test " + [guid]::NewGuid())
New-Item -ItemType Directory $root | Out-Null
$archive = Join-Path $root 'cache/package.zip'
$fixture = Join-Path $root 'fixture'
[IO.File]::WriteAllText($fixture, 'verified payload')
$hash = (Get-FileHash $fixture -Algorithm SHA256).Hash
$script:calls = 0
$script:payload = 'verified payload'
$script:downloadExitCode = 0
function curl.exe {
    $script:calls++
    $outputIndex = [array]::IndexOf($args, '--output')
    if ($outputIndex -lt 0) { throw 'Missing download destination' }
    [IO.File]::WriteAllText($args[$outputIndex + 1], $script:payload)
    $global:LASTEXITCODE = $script:downloadExitCode
}
function Assert-True($condition, $message) {
    if (-not $condition) { throw $message }
}
function Download {
    Get-WindowsDependency -URL 'https://example.invalid/package.zip' -SHA256 $hash -Archive $archive
}
function Assert-Rejected($expected) {
    $errorMessage = ''
    try { Download } catch { $errorMessage = $_.Exception.Message }
    Assert-True ($errorMessage.StartsWith($expected)) "Expected $expected, got: $errorMessage"
    Assert-True (-not (Test-Path $archive)) 'Failed download became a cached archive'
    Assert-True (-not (Test-Path "$archive.partial")) 'Partial download was retained'
}
try {
    Download
    Assert-True ($script:calls -eq 1) 'Cold cache did not download'
    Assert-True ((Get-FileHash $archive -Algorithm SHA256).Hash -eq $hash) 'Wrong cached contents'

    $script:downloadExitCode = 22
    Download
    Assert-True ($script:calls -eq 1) 'Warm cache attempted network access'

    [IO.File]::WriteAllText($archive, 'corrupt cache')
    $script:downloadExitCode = 0
    Download
    Assert-True ($script:calls -eq 2) 'Corrupt cache was not replaced'
    Assert-True ((Get-FileHash $archive -Algorithm SHA256).Hash -eq $hash) 'Corrupt cache was trusted'

    Remove-Item $archive
    $script:payload = 'invalid response'
    Assert-Rejected 'SHA-256 mismatch:'

    $script:payload = 'verified payload'
    $script:downloadExitCode = 22
    Assert-Rejected 'Download failed:'
    $global:LASTEXITCODE = 0
    Write-Host 'Windows dependency download tests passed.'
} finally {
    Remove-Item $root -Recurse -Force
}
