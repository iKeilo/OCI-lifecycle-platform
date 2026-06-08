param(
  [string]$ApiUrl = "http://localhost:8080",
  [string]$CompartmentId = "",
  [string]$SubnetId = "",
  [string]$ImageId = "",
  [int64]$BootVolumeGb = 10
)

$ErrorActionPreference = "Stop"

Write-Host "[1/2] OCI readiness"
$readiness = Invoke-RestMethod "$ApiUrl/api/oci/readiness"
if (-not $readiness.ready) {
  $missing = ""
  if ($readiness.missing) {
    $missing = " Missing: " + ($readiness.missing -join ", ")
  }
  throw "OCI is not ready for real E3 Flex lifecycle validation. $($readiness.message)$missing"
}

Write-Host "[2/2] Create VM.Standard.E3.Flex 1C/1G and run lifecycle validation"
$payload = @{
  compartmentId = $CompartmentId
  subnetId = $SubnetId
  imageId = $ImageId
  bootVolumeGb = $BootVolumeGb
} | ConvertTo-Json

try {
  $result = Invoke-RestMethod -Method Post -Uri "$ApiUrl/api/oci/smoke/e3-flex-lifecycle" -ContentType "application/json" -Body $payload
} catch {
  $body = $_.ErrorDetails.Message
  if (-not $body -and $_.Exception.Response) {
    $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
    $body = $reader.ReadToEnd()
  }
  if ($body) {
    $result = $body | ConvertFrom-Json
  } else {
    throw
  }
}

Write-Host "Verified: $($result.verified)"
Write-Host "Display name: $($result.displayName)"
Write-Host "Instance OCID: $($result.instanceId)"
Write-Host "Final state: $($result.finalState)"
foreach ($step in $result.steps) {
  Write-Host "$($step.name) [$($step.operation)] verified=$($step.verified) state=$($step.state) requestId=$($step.requestId) workRequestId=$($step.workRequestId)"
  if ($step.errorCode) {
    Write-Host "  error=$($step.errorCode) $($step.errorMessage)"
  }
}

if (-not $result.verified) {
  throw "E3 Flex lifecycle validation failed: $($result.errorCode) $($result.errorMessage)"
}
