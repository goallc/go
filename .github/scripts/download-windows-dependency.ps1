# Cache only verified archives, never partially downloaded files.
function Get-WindowsDependency {
    param([string]$URL, [string]$SHA256, [string]$Archive)

    if (Test-Path $Archive) {
        if ((Get-FileHash $Archive -Algorithm SHA256).Hash -eq $SHA256) {
            Write-Host "Using verified archive: $Archive"
            return
        }
        Remove-Item $Archive -Force
    }
    New-Item -ItemType Directory -Force (Split-Path $Archive) | Out-Null
    $partial = "$Archive.partial"
    try {
        & curl.exe --fail --location --retry 5 --retry-delay 10 --retry-max-time 600 --connect-timeout 30 --max-time 300 --output $partial $URL
        if ($LASTEXITCODE -ne 0) { throw "Download failed: $URL" }
        if ((Get-FileHash $partial -Algorithm SHA256).Hash -ne $SHA256) {
            throw "SHA-256 mismatch: $URL"
        }
        Move-Item $partial $Archive -Force
    } finally {
        if (Test-Path $partial) { Remove-Item $partial -Force }
    }
}
