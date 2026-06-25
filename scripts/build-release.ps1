# Builds a portable Windows release ZIP for d2rbot.
param(
    [string]$Version = ""
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

function Resolve-GoExe {
    if ($env:GOEXE -and (Test-Path $env:GOEXE)) {
        return $env:GOEXE
    }
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($go) {
        return $go.Source
    }
    $default = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path $default) {
        return $default
    }
    throw "go not found; install Go or set GOEXE"
}

$GoExe = Resolve-GoExe

if (-not $Version) {
    $match = Select-String -Path "internal/version/version.go" -Pattern 'Version\s*=\s*"([^"]+)"' |
        Select-Object -First 1
    if (-not $match) {
        throw "could not read version from internal/version/version.go"
    }
    $Version = $match.Matches.Groups[1].Value
}

$Commit = "dev"
try {
    $Commit = (git rev-parse --short HEAD 2>$null)
    if (-not $Commit) {
        $Commit = "dev"
    }
} catch {
    $Commit = "dev"
}

$Name = "d2rbot-v$Version-windows-amd64"
$OutDir = Join-Path "dist" $Name
$ZipPath = Join-Path "dist" "$Name.zip"

Remove-Item -Recurse -Force $OutDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path (Join-Path $OutDir "configs") | Out-Null

$LdFlags = @(
    "-s"
    "-w"
    "-X github.com/Tyniann/d2r-offline-farming-bot/internal/version.Version=$Version"
    "-X github.com/Tyniann/d2r-offline-farming-bot/internal/version.Commit=$Commit"
) -join " "

& $GoExe build -ldflags $LdFlags -o (Join-Path $OutDir "d2rbot.exe") ./cmd/d2rbot
if ($LASTEXITCODE -ne 0) {
    throw "go build failed with exit code $LASTEXITCODE"
}

Copy-Item "configs/config.example.yaml" (Join-Path $OutDir "configs/")
Copy-Item "configs/offsets.example.yaml" (Join-Path $OutDir "configs/")

@"
D2R Offline Farming Bot v$Version ($Commit)

1. Copy configs\config.example.yaml to configs\config.yaml and adjust settings.
2. Optional: copy configs\offsets.example.yaml to configs\offsets.local.yaml.
3. Run d2rbot.exe from this folder (or pass --config).
4. Use --probe for world-state logs while testing.

Requires Windows and D2R offline/singleplayer.
Run with the same privileges as D2R if memory reads fail.
"@ | Set-Content -Encoding utf8 (Join-Path $OutDir "INSTALL.txt")

if (Test-Path $ZipPath) {
    Remove-Item $ZipPath
}
Compress-Archive -Path $OutDir -DestinationPath $ZipPath

Write-Host "Release built: $ZipPath"
