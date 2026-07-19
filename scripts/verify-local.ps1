param(
    [ValidateSet('changed','quick','full','backend','database','frontend','contracts','security')]
    [string]$Mode = 'changed',
    [string]$Base = '',
    [switch]$Ci,
    [switch]$KeepDatabase
)

$bash = Get-Command bash -ErrorAction SilentlyContinue
if (-not $bash) {
    Write-Error 'Install Git Bash or WSL before running local verification.'
    exit 1
}

$argsList = @('scripts/verify-local.sh', $Mode)
if ($Base) { $argsList += @('--base', $Base) }
if ($Ci) { $argsList += '--ci' }
if ($KeepDatabase) { $argsList += '--keep-database' }

Push-Location (Resolve-Path (Join-Path $PSScriptRoot '..'))
try {
    & $bash.Source @argsList
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
