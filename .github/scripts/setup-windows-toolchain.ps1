$ErrorActionPreference = 'Stop'
$packages = @(
    @{
        Name = 'llvm-mingw-20261020-ucrt-x86_64'
        URL = 'https://github.com/mstorsjo/llvm-mingw/releases/download/20261020/llvm-mingw-20261020-ucrt-x86_64.zip'
        SHA256 = 'fd5462b7f5d93b2ff981fcd3f9c7e427e47f3203c64ea91c27bcc444eebd74cd'
    },
    @{
        Name = 'ccache-4.14.2-windows-x86_64'
        URL = 'https://github.com/ccache/ccache/releases/download/v4.14.2/ccache-4.14.2-windows-x86_64.zip'
        SHA256 = '6993ce645e53192cec592ee23186200b574ac4c59097ecde8d9cfd681b47ae84'
    }
)
foreach ($package in $packages) {
    $archive = Join-Path $env:RUNNER_TEMP ($package.Name + '.zip')
    & curl.exe --fail --location --retry 5 --output $archive $package.URL
    if ($LASTEXITCODE -ne 0) { throw "Download failed: $($package.Name)" }
    if ((Get-FileHash $archive -Algorithm SHA256).Hash -ne $package.SHA256) {
        throw "SHA-256 mismatch: $($package.Name)"
    }
    Expand-Archive $archive -DestinationPath $env:RUNNER_TEMP -Force
}
$bin = "$env:RUNNER_TEMP/$($packages[0].Name)/bin".Replace('\', '/')
$cache = "$env:RUNNER_TEMP/$($packages[1].Name)".Replace('\', '/')
$bin, $cache | Out-File $env:GITHUB_PATH -Append -Encoding utf8
"CC=$bin/clang.exe", "CXX=$bin/clang++.exe", "GOALLC_CCACHE=$cache/ccache.exe", "GOALLC_TOOLCHAIN_LICENSE=$bin/../LICENSE.TXT" |
    Out-File $env:GITHUB_ENV -Append -Encoding utf8
