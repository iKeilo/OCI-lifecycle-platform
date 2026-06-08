param(
  [string]$ApiUrl = "http://localhost:8080"
)

$ErrorActionPreference = "Stop"
$Root = Split-Path -Parent $PSScriptRoot

Write-Host "[1/4] Go tests"
Push-Location (Join-Path $Root "backend")
go test ./...
Pop-Location

Write-Host "[2/4] Frontend build"
Push-Location $Root
npm run build
Pop-Location

Write-Host "[3/4] API health"
$health = Invoke-RestMethod "$ApiUrl/api/health"
if ($health.status -ne "ok") {
  throw "API health check failed"
}
if ($health.ociApiVerified -ne $false) {
  throw "local verification must not report OCI API verification"
}

Write-Host "[4/4] Launch options"
$options = Invoke-RestMethod "$ApiUrl/api/launch-options"
if (
  $null -eq $options.profiles -or
  $null -eq $options.templates -or
  $null -eq $options.regions -or
  $null -eq $options.compartments -or
  $null -eq $options.availabilityAds -or
  $null -eq $options.images -or
  $null -eq $options.shapes -or
  $null -eq $options.vcns -or
  $null -eq $options.subnets -or
  $null -eq $options.reservedIps
) {
  throw "launch options response is malformed"
}

Write-Host "Engineering verification passed"
Write-Host "Note: this script does not verify real OCI API behavior."
