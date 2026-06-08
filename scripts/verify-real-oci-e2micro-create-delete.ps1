param(
  [string]$ApiUrl = "http://localhost:8080",
  [string]$CompartmentId = "",
  [string]$SubnetId = "",
  [string]$ImageId = ""
)

$ErrorActionPreference = "Stop"

Write-Host "[1/3] OCI readiness"
$readiness = Invoke-RestMethod "$ApiUrl/api/oci/readiness"
if (-not $readiness.ready) {
  $missing = ""
  if ($readiness.missing) {
    $missing = " Missing: " + ($readiness.missing -join ", ")
  }
  throw "OCI is not ready for real API validation. $($readiness.message)$missing"
}

Write-Host "[2/3] Create and delete VM.Standard.E2.1.Micro smoke instance"
$payload = @{
  compartmentId = $CompartmentId
  subnetId = $SubnetId
  imageId = $ImageId
} | ConvertTo-Json

try {
  $result = Invoke-RestMethod -Method Post -Uri "$ApiUrl/api/oci/smoke/e2-micro-create-delete" -ContentType "application/json" -Body $payload
} catch {
  $body = $_.ErrorDetails.Message
  if ($body) {
    $result = $body | ConvertFrom-Json
  } else {
    throw
  }
}

if (-not $result.verified) {
  Write-Host "Verified: false"
  Write-Host "Compartment: $($result.compartmentId)"
  Write-Host "Availability domain: $($result.availabilityDomain)"
  Write-Host "Subnet: $($result.subnetId)"
  Write-Host "Image: $($result.imageId)"
  Write-Host "Display name: $($result.displayName)"
  Write-Host "Launch request id: $($result.launchRequestId)"
  Write-Host "Cleanup attempted: $($result.cleanupAttempted)"
  Write-Host "Cleanup succeeded: $($result.cleanupSucceeded)"
  throw "E2.1.Micro create/delete smoke failed: $($result.errorCode) $($result.errorMessage)"
}

Write-Host "[3/3] Smoke validation passed"
Write-Host "Display name: $($result.displayName)"
Write-Host "Instance OCID: $($result.instanceId)"
Write-Host "Launch request id: $($result.launchRequestId)"
Write-Host "Launch work request id: $($result.launchWorkRequestId)"
Write-Host "Terminate request id: $($result.terminateRequestId)"
Write-Host "Final state: $($result.finalState)"
