$ErrorActionPreference = "Stop"

$Repo = "madLinux7/dssh"
$InstallDir = if ($env:INSTALL_DIR) { $env:INSTALL_DIR } else { "$env:LOCALAPPDATA\dssh" }
$Binary = "dssh.exe"

Write-Host "Fetching latest release..."
$Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$Tag = $Release.tag_name
if (-not $Tag) {
    Write-Error "Could not determine latest release"
    exit 1
}

$Url = "https://github.com/$Repo/releases/download/$Tag/dssh.exe"
Write-Host "Downloading dssh $Tag..."

New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
$OutFile = Join-Path $InstallDir $Binary
Invoke-WebRequest -Uri $Url -OutFile $OutFile

Write-Host "dssh $Tag installed to $OutFile"

$UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("Path", "$InstallDir;$UserPath", "User")
    Write-Host "$InstallDir added to your PATH. Restart your terminal to use dssh."
}
