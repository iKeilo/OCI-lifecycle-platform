param(
  [string]$ApiUrl = "http://localhost:8080",
  [string]$CompartmentId = ""
)

$ErrorActionPreference = "Stop"

Write-Host "[1/2] OCI readiness"
$readiness = Invoke-RestMethod "$ApiUrl/api/oci/readiness"
if (-not $readiness.ready) {
  $missing = ""
  if ($readiness.missing) {
    $missing = " Missing: " + ($readiness.missing -join ", ")
  }
  throw "OCI is not ready for real API validation. $($readiness.message)$missing"
}

Write-Host "[2/2] Read-only OCI SDK validation"
$payload = @{ compartmentId = $CompartmentId } | ConvertTo-Json
$result = Invoke-RestMethod -Method Post -Uri "$ApiUrl/api/oci/validate-readonly" -ContentType "application/json" -Body $payload
if (-not $result.verified) {
  throw "Read-only OCI validation failed: $($result.errorCode) $($result.errorMessage)"
}

Write-Host "Read-only OCI validation passed"
Write-Host "Region request id: $($result.regionRequestId)"
Write-Host "Instances request id: $($result.instancesRequestId)"
Write-Host "Regions: $($result.regions.Count)"
Write-Host "Instances returned: $($result.instances.Count)"
