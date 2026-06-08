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
  throw "OCI is not ready for smoke cleanup. $($readiness.message)$missing"
}

Write-Host "[2/2] Cleanup codex smoke instances"
$payload = @{
  compartmentId = $CompartmentId
} | ConvertTo-Json

$result = Invoke-RestMethod -Method Post -Uri "$ApiUrl/api/oci/smoke/cleanup" -ContentType "application/json" -Body $payload
if (-not $result.verified) {
  throw "Smoke cleanup incomplete: $($result.errorCode) $($result.errorMessage)"
}

Write-Host "Smoke cleanup passed"
foreach ($item in $result.items) {
  Write-Host "$($item.displayName) $($item.initialState) -> $($item.finalState) terminateRequestId=$($item.terminateRequestId)"
}
