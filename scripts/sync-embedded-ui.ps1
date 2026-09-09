param(
    [switch]$Check
)

$ErrorActionPreference = "Stop"
$Root = [IO.Path]::GetFullPath((Split-Path -Parent $PSScriptRoot))
$WebRoot = Join-Path $Root "web"
$EmbeddedRoot = Join-Path $Root "internal\api\ui\dist"
$CheckRoot = Join-Path $Root ".tmp\embedded-ui-check-$PID"

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

function Get-TreeHashes([string]$Path) {
    if (-not (Test-Path -LiteralPath $Path -PathType Container)) {
        throw "UI bundle directory is missing: $Path"
    }
    $prefix = [IO.Path]::GetFullPath($Path).TrimEnd('\') + '\'
    $hashes = @{}
    Get-ChildItem -LiteralPath $Path -Recurse -File | ForEach-Object {
        $relative = $_.FullName.Substring($prefix.Length).Replace('\', '/')
        $hashes[$relative] = (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash
    }
    return $hashes
}

function Assert-SameTree([string]$CommittedPath, [string]$GeneratedPath) {
    $committed = Get-TreeHashes $CommittedPath
    $generated = Get-TreeHashes $GeneratedPath
    $paths = @($committed.Keys) + @($generated.Keys) | Sort-Object -Unique
    $differences = @($paths | Where-Object {
        -not $committed.ContainsKey($_) -or
        -not $generated.ContainsKey($_) -or
        $committed[$_] -ne $generated[$_]
    })
    if ($differences.Count -ne 0) {
        $sample = ($differences | Select-Object -First 20) -join ", "
        throw "embedded UI differs from web sources: $sample. Run scripts/sync-embedded-ui.ps1 and commit internal/api/ui/dist before releasing."
    }
}

Invoke-Checked "pnpm" @("install", "--frozen-lockfile") $WebRoot

if (-not $Check) {
    Invoke-Checked "pnpm" @("generate") $WebRoot
    Invoke-Checked "pnpm" @("typecheck:renderer") $WebRoot
    Invoke-Checked "pnpm" @("build:renderer") $WebRoot
    Write-Host "Embedded UI synchronized: $EmbeddedRoot"
    exit 0
}

if (Test-Path -LiteralPath $CheckRoot) {
    Remove-Item -LiteralPath $CheckRoot -Recurse -Force
}
try {
    Invoke-Checked "node" @((Join-Path $WebRoot "scripts\generate-api.mjs"), "--check") $WebRoot
    Invoke-Checked "pnpm" @("typecheck:renderer") $WebRoot
    Invoke-Checked "pnpm" @("exec", "vite", "build", "--outDir", $CheckRoot, "--emptyOutDir") $WebRoot
    Assert-SameTree $EmbeddedRoot $CheckRoot
    Write-Host "Embedded UI matches web sources."
} finally {
    if (Test-Path -LiteralPath $CheckRoot) {
        Remove-Item -LiteralPath $CheckRoot -Recurse -Force
    }
}
