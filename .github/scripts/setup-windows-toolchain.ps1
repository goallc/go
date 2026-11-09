$ErrorActionPreference = 'Stop'
. "$PSScriptRoot/download-windows-dependency.ps1"
$downloads = Join-Path $env:RUNNER_TEMP 'goallc-windows-downloads'
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
    },
    @{
        Name = 'zstd-v1.5.7-win64'
        URL = 'https://github.com/facebook/zstd/releases/download/v1.5.7/zstd-v1.5.7-win64.zip'
        SHA256 = 'acb4e8111511749dc7a3ebedca9b04190e37a17afeb73f55d4425dbf0b90fad9'
    }
)
foreach ($package in $packages) {
    $archive = Join-Path $downloads ($package.Name + '.zip')
    Get-WindowsDependency -URL $package.URL -SHA256 $package.SHA256 -Archive $archive
    Expand-Archive $archive -DestinationPath $env:RUNNER_TEMP -Force
}
# Use a native pkg-config executable; the runner's Strawberry Perl .bat
# wrapper does not work when invoked by the Go command's cgo tests.
$pkgconfArchive = Join-Path $downloads 'pkgconf-x64-3.0.10.msi'
$pkgconfRoot = Join-Path $env:RUNNER_TEMP 'pkgconf-3.0.10'
Get-WindowsDependency -Archive $pkgconfArchive `
    -URL 'https://github.com/pkgconf/pkgconf/releases/download/pkgconf-3.0.10/pkgconf-x64-3.0.10.msi' `
    -SHA256 '28cc9e44473a3ef277260f3dac20f58e2a667760393bd47dc309b6426e9bd727'
$install = Start-Process msiexec.exe -ArgumentList @('/a', ('"' + $pkgconfArchive + '"'), '/qn', ('TARGETDIR="' + $pkgconfRoot + '"')) -Wait -PassThru
if ($install.ExitCode -ne 0) { throw "pkgconf extraction failed: $($install.ExitCode)" }
$pkgconf = Get-ChildItem $pkgconfRoot -Recurse -Filter pkg-config.exe | Select-Object -First 1
if (-not $pkgconf) { throw 'pkg-config.exe is missing from the pkgconf package' }
& $pkgconf.FullName --version
if ($LASTEXITCODE -ne 0) { throw 'pkg-config.exe could not run' }
$pkgconf.DirectoryName | Out-File $env:GITHUB_PATH -Append -Encoding utf8
$bin = "$env:RUNNER_TEMP/$($packages[0].Name)/bin".Replace('\', '/')
$cache = "$env:RUNNER_TEMP/$($packages[1].Name)".Replace('\', '/')
$zstd = "$env:RUNNER_TEMP/$($packages[2].Name)".Replace('\', '/')
$bin, $cache, $zstd | Out-File $env:GITHUB_PATH -Append -Encoding utf8
# dist records CC/CXX as the installed SDK's defaults. Resolve these through
# PATH so a relocated SDK does not retain the build runner's temporary path.
"CC=clang", "CXX=clang++", "GOALLC_CCACHE=$cache/ccache.exe", "GOALLC_TOOLCHAIN_LICENSE=$bin/../LICENSE.TXT" |
    Out-File $env:GITHUB_ENV -Append -Encoding utf8
