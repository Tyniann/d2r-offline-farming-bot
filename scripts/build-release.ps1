# Builds, inspects and smoke-tests the per-user Windows NSIS product.
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$')]
    [string]$Version,
    [switch]$SkipAutomatedChecks,
    [switch]$SkipProductSmoke
)

$ErrorActionPreference = "Stop"
$Root = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$WebRoot = Join-Path $Root "web"
$ReleaseRoot = Join-Path $Root "dist\release"
$BuilderRoot = Join-Path $Root "dist\electron-builder"
$ResourcesRoot = Join-Path $WebRoot "release-resources"
$SmokeRoot = Join-Path $Root ".tmp\release-smoke-$PID"

function Assert-WorkspaceChild([string]$Path, [string]$ExpectedParent) {
    $resolved = [IO.Path]::GetFullPath($Path)
    $parent = [IO.Path]::GetFullPath($ExpectedParent).TrimEnd('\') + '\'
    if (-not $resolved.StartsWith($parent, [StringComparison]::OrdinalIgnoreCase)) {
        throw "refusing filesystem mutation outside $parent`: $resolved"
    }
}

function Invoke-Checked([string]$Program, [string[]]$Arguments, [string]$WorkingDirectory = $Root) {
    Push-Location $WorkingDirectory
    try {
        & $Program @Arguments
        if ($LASTEXITCODE -ne 0) {
            throw "$Program failed with exit code $LASTEXITCODE"
        }
    } finally {
        Pop-Location
    }
}

function Invoke-GUIProcess([string]$Program, [string[]]$Arguments) {
    $process = Start-Process -FilePath $Program -ArgumentList $Arguments -Wait -PassThru -WindowStyle Hidden
    if ($process.ExitCode -ne 0) {
        throw "$Program failed with exit code $($process.ExitCode)"
    }
}

function Resolve-GoExe {
    if ($env:GOEXE -and (Test-Path -LiteralPath $env:GOEXE -PathType Leaf)) {
        return $env:GOEXE
    }
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($go) {
        return $go.Source
    }
    $default = "C:\Program Files\Go\bin\go.exe"
    if (Test-Path -LiteralPath $default -PathType Leaf) {
        return $default
    }
    throw "go not found; install Go or set GOEXE"
}

foreach ($path in @($ReleaseRoot, $BuilderRoot, $ResourcesRoot, $SmokeRoot)) {
    Assert-WorkspaceChild $path $Root
}

$Commit = (git -C $Root rev-parse --short HEAD 2>$null).Trim()
if (-not $Commit -or $Commit -eq "dev") {
    throw "release build requires a readable non-dev Git commit"
}
$GoExe = Resolve-GoExe
$OriginalLocalAppData = $env:LOCALAPPDATA

foreach ($path in @($ReleaseRoot, $BuilderRoot, $ResourcesRoot, $SmokeRoot)) {
    if (Test-Path -LiteralPath $path) {
        Remove-Item -LiteralPath $path -Recurse -Force
    }
}
New-Item -ItemType Directory -Path $ReleaseRoot, (Join-Path $ResourcesRoot "core"), $SmokeRoot -Force | Out-Null

try {
    Invoke-Checked "pnpm" @("install", "--frozen-lockfile") $WebRoot
    Invoke-Checked "pnpm" @("generate") $WebRoot
    Invoke-Checked "pnpm" @("build") $WebRoot

    if (-not $SkipAutomatedChecks) {
        Invoke-Checked "pnpm" @("test") $WebRoot
        Invoke-Checked "pnpm" @("typecheck") $WebRoot

        $effectiveUser = (whoami).Trim()
        if (-not $effectiveUser) {
            throw "native Electron checks require a resolved effective Windows user"
        }
        Write-Host "Native Electron checks run as $effectiveUser"
        Invoke-Checked "pnpm" @("test:electron") $WebRoot

        Invoke-Checked $GoExe @("test", "./...")
        Invoke-Checked "golangci-lint" @("run")
    }

    $DefaultsRoot = Join-Path $ResourcesRoot "defaults"
    Invoke-Checked $GoExe @("run", "./tools/build-default-bundle", "--source", (Join-Path $Root "configs"), "--output", $DefaultsRoot)

    $CorePath = Join-Path $ResourcesRoot "core\d2rbot.exe"
    $LdFlags = "-s -w -X github.com/Tyniann/d2r-offline-farming-bot/internal/version.Version=$Version -X github.com/Tyniann/d2r-offline-farming-bot/internal/version.Commit=$Commit"
    Invoke-Checked $GoExe @("build", "-trimpath", "-ldflags", $LdFlags, "-o", $CorePath, "./cmd/d2rbot")
    $coreVersion = (& $CorePath --version).Trim()
    if ($LASTEXITCODE -ne 0 -or $coreVersion -ne "d2rbot $Version ($Commit)" -or $coreVersion.Contains("(dev)")) {
        throw "Core version verification failed: $coreVersion"
    }

    Invoke-Checked "pnpm" @("build:icon") $WebRoot
    Invoke-Checked "pnpm" @(
        "exec", "electron-builder", "--win", "nsis", "--x64", "--publish", "never",
        "--config.extraMetadata.version=$Version",
        "--config.directories.output=$BuilderRoot"
    ) $WebRoot

    $installers = @(Get-ChildItem -LiteralPath $BuilderRoot -File -Filter "*-Setup.exe")
    if ($installers.Count -ne 1) {
        throw "electron-builder produced $($installers.Count) setup executables, expected exactly one"
    }
    $Installer = $installers[0].FullName
    $Unpacked = Join-Path $BuilderRoot "win-unpacked"
    $PackagedExe = Join-Path $Unpacked "D2R Offline Farming Bot.exe"
    $PackagedCore = Join-Path $Unpacked "resources\core\d2rbot.exe"
    foreach ($required in @($PackagedExe, $PackagedCore, (Join-Path $Unpacked "resources\defaults\bundle.json"), (Join-Path $Unpacked "resources\INSTALLATION.md"))) {
        if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
            throw "packaged artifact is missing $required"
        }
    }

    $forbidden = @(
        Get-ChildItem -LiteralPath $Unpacked -Recurse -Force |
            Where-Object {
                $_.FullName -match '[\\/](node_modules|\.git|logs[\\/]telemetry|diagnostics)([\\/]|$)' -or
                $_.Name -match '^(\.env|config\.local\.yaml|offsets\.scanned\.yaml|desktop-settings\.json)$'
            }
    )
    if ($forbidden.Count -ne 0) {
        throw "packaged artifact contains forbidden workspace/local content: $($forbidden[0].FullName)"
    }
    Invoke-Checked "node" @((Join-Path $WebRoot "electron\verify-package.mjs"), (Join-Path $Unpacked "resources\app.asar"), $Version) $WebRoot
    $secretMatches = @(
        Get-ChildItem -LiteralPath (Join-Path $Unpacked "resources\defaults") -Recurse -File |
            Select-String -Pattern '(?i)(control_token|authorization\s*:|password\s*:\s*\S+|secret\s*:\s*\S+)' -ErrorAction Stop
    )
    if ($secretMatches.Count -ne 0) {
        throw "packaged defaults contain a secret-like value"
    }

    $packagedProductVersion = (Get-Item -LiteralPath $PackagedExe).VersionInfo.ProductVersion
    if (-not $packagedProductVersion.StartsWith($Version, [StringComparison]::Ordinal)) {
        throw "packaged App version $packagedProductVersion differs from $Version"
    }
    $packagedCoreVersion = (& $PackagedCore --version).Trim()
    if ($packagedCoreVersion -ne "d2rbot $Version ($Commit)" -or $packagedCoreVersion.Contains("(dev)")) {
        throw "packaged Core version verification failed: $packagedCoreVersion"
    }

    if (-not $SkipProductSmoke) {
        $ProfileRoot = Join-Path $SmokeRoot "profile"
        $LocalAppData = Join-Path $ProfileRoot "AppData\Local"
        $InstallRoot = Join-Path $SmokeRoot "install"
        $DataRoot = Join-Path $LocalAppData "D2ROfflineFarmingBot"
        New-Item -ItemType Directory -Path $LocalAppData -Force | Out-Null
        $env:LOCALAPPDATA = $LocalAppData

        Invoke-GUIProcess $Installer @("/S", "/D=$InstallRoot")
        $InstalledExe = Join-Path $InstallRoot "D2R Offline Farming Bot.exe"
        $InstalledCore = Join-Path $InstallRoot "resources\core\d2rbot.exe"
        if (-not (Test-Path -LiteralPath $InstalledExe -PathType Leaf)) {
            throw "silent per-user installation did not create the App executable"
        }
        Invoke-Checked "node" @((Join-Path $WebRoot "electron\package-smoke.mjs"), $InstalledExe, $DataRoot, $Version, $LocalAppData) $WebRoot
        Set-Content -LiteralPath (Join-Path $DataRoot "upgrade-uninstall-sentinel.txt") -Value "preserve" -Encoding utf8

        Invoke-GUIProcess $Installer @("/S", "/D=$InstallRoot")
        if (-not (Test-Path -LiteralPath (Join-Path $DataRoot "upgrade-uninstall-sentinel.txt") -PathType Leaf)) {
            throw "upgrade changed the existing data root"
        }
        $Uninstaller = Join-Path $InstallRoot "Uninstall D2R Offline Farming Bot.exe"
        if (-not (Test-Path -LiteralPath $Uninstaller -PathType Leaf)) {
            throw "uninstaller is missing"
        }
        Invoke-GUIProcess $Uninstaller @("/S")
        for ($attempt = 0; $attempt -lt 20 -and (Test-Path -LiteralPath $InstalledExe); $attempt++) {
            Start-Sleep -Milliseconds 250
        }
        if (Test-Path -LiteralPath $InstalledExe) {
            throw "silent uninstall left the installed App executable behind"
        }
        if (-not (Test-Path -LiteralPath (Join-Path $DataRoot "upgrade-uninstall-sentinel.txt") -PathType Leaf)) {
            throw "silent default uninstall removed the data root"
        }
    }

    $FinalInstaller = Join-Path $ReleaseRoot $installers[0].Name
    Copy-Item -LiteralPath $Installer -Destination $FinalInstaller
    $Hash = (Get-FileHash -LiteralPath $FinalInstaller -Algorithm SHA256).Hash.ToLowerInvariant()
    $ChecksumPath = "$FinalInstaller.sha256"
    Set-Content -LiteralPath $ChecksumPath -Value "$Hash *$([IO.Path]::GetFileName($FinalInstaller))" -Encoding ascii
    $finalFiles = @(Get-ChildItem -LiteralPath $ReleaseRoot -File)
    if ($finalFiles.Count -ne 2 -or @($finalFiles | Where-Object Name -like "*-Setup.exe").Count -ne 1 -or @($finalFiles | Where-Object Name -like "*.sha256").Count -ne 1) {
        throw "release output must contain exactly one installer and one checksum"
    }
    $result = if ($SkipProductSmoke) { "built and statically verified" } else { "built and smoke-tested" }
    Write-Host "Release ${result}: $FinalInstaller"
} finally {
    $env:LOCALAPPDATA = $OriginalLocalAppData
    foreach ($path in @($BuilderRoot, $ResourcesRoot, $SmokeRoot)) {
        if (Test-Path -LiteralPath $path) {
            Assert-WorkspaceChild $path $Root
            Remove-Item -LiteralPath $path -Recurse -Force
        }
    }
}
