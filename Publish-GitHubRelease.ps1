param(
  [string]$Version = "",
  [string]$Remote = "origin",
  [string]$Branch = "main",
  [string]$WorkflowName = "Build and Push Images",
  [string[]]$RequiredAssets = @(),
  [int]$TimeoutMinutes = 90,
  [switch]$CommitAll,
  [string]$CommitMessage = "",
  [switch]$SkipWait
)

$ErrorActionPreference = "Stop"

function Invoke-CheckedGit {
  param([Parameter(ValueFromRemainingArguments = $true)][string[]]$Args)
  & git @Args
  if ($LASTEXITCODE -ne 0) {
    throw "git $($Args -join ' ') failed with exit code $LASTEXITCODE"
  }
}

function Get-GitHubRepoPath {
  param([string]$RemoteName)
  $url = (& git remote get-url $RemoteName).Trim()
  if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($url)) {
    throw "Unable to read git remote '$RemoteName'."
  }
  if ($url -match "github\.com[:/](?<owner>[^/]+)/(?<repo>[^/.]+)(\.git)?$") {
    return "$($Matches.owner)/$($Matches.repo)"
  }
  throw "Remote '$RemoteName' does not look like a GitHub repository: $url"
}

function Invoke-GitHubApi {
  param([string]$Uri)
  $headers = @{ "User-Agent" = "codex-github-publish-workflow" }
  $token = $env:GITHUB_TOKEN
  if ([string]::IsNullOrWhiteSpace($token)) {
    $token = $env:GH_TOKEN
  }
  if (-not [string]::IsNullOrWhiteSpace($token)) {
    $headers["Authorization"] = "Bearer $token"
  }
  Invoke-RestMethod -Uri $Uri -Headers $headers
}

function Convert-VersionParts {
  param([string]$Tag)
  $normalized = $Tag.TrimStart("v")
  if ($normalized -notmatch "^\d+(\.\d+){0,3}([-.].*)?$") {
    return $null
  }
  try {
    return [version](($normalized -split "[-+]")[0])
  } catch {
    return $null
  }
}

function Get-NextPatchVersion {
  param([string]$RepoPath)
  $tags = @()
  try {
    $releases = Invoke-GitHubApi "https://api.github.com/repos/$RepoPath/releases?per_page=50"
    $tags += @($releases | ForEach-Object { $_.tag_name })
  } catch {
    Write-Warning "Could not read releases from GitHub, falling back to local tags: $($_.Exception.Message)"
  }

  $tags += @(& git tag --list)
  $latest = $tags |
    Where-Object { -not [string]::IsNullOrWhiteSpace($_) } |
    ForEach-Object {
      $parsed = Convert-VersionParts $_
      if ($null -ne $parsed) {
        [pscustomobject]@{ Tag = $_; Version = $parsed }
      }
    } |
    Sort-Object Version -Descending |
    Select-Object -First 1

  if ($null -eq $latest) {
    return "1.0.0"
  }

  return "$($latest.Version.Major).$($latest.Version.Minor).$($latest.Version.Build + 1)"
}

function Wait-GitHubWorkflow {
  param([string]$RepoPath, [string]$Tag, [string]$HeadSha, [string]$Name, [int]$Minutes)
  $deadline = (Get-Date).AddMinutes($Minutes)
  $run = $null

  while ((Get-Date) -lt $deadline) {
    $runs = Invoke-GitHubApi "https://api.github.com/repos/$RepoPath/actions/runs?per_page=30"
    $run = @($runs.workflow_runs | Where-Object {
      $_.name -eq $Name -and
      $_.head_branch -eq $Tag -and
      $_.head_sha -eq $HeadSha
    } | Select-Object -First 1)

    if ($run) {
      Write-Host "Workflow: $($run.status) / $($run.conclusion) - $($run.html_url)"
      if ($run.status -eq "completed") {
        if ($run.conclusion -ne "success") {
          throw "Release workflow failed with conclusion '$($run.conclusion)': $($run.html_url)"
        }
        return $run
      }
    } else {
      Write-Host "Waiting for workflow '$Name' to appear for tag $Tag..."
    }

    Start-Sleep -Seconds 20
  }

  throw "Timed out waiting for workflow '$Name' after $Minutes minutes."
}

function Test-GitHubReleaseAssets {
  param([string]$RepoPath, [string]$Tag, [string[]]$Required)
  $release = Invoke-GitHubApi "https://api.github.com/repos/$RepoPath/releases/tags/$Tag"
  if ($Required.Count -gt 0) {
    $assetNames = @($release.assets | ForEach-Object { $_.name })
    $missing = @($Required | Where-Object { $assetNames -notcontains $_ })
    if ($missing.Count -gt 0) {
      throw "Release $Tag is missing assets: $($missing -join ', ')"
    }
  }
  return $release
}

$repoPath = Get-GitHubRepoPath $Remote
$currentBranch = (& git rev-parse --abbrev-ref HEAD).Trim()
if ($currentBranch -ne $Branch) {
  throw "Current branch is '$currentBranch', expected '$Branch'."
}

if ($CommitAll) {
  if ([string]::IsNullOrWhiteSpace($CommitMessage)) {
    throw "-CommitMessage is required when using -CommitAll."
  }
  Invoke-CheckedGit -Args @("add", "-A")
  $pending = (& git status --short)
  if (-not [string]::IsNullOrWhiteSpace($pending)) {
    Invoke-CheckedGit -Args @("commit", "-m", $CommitMessage)
  }
}

$dirty = (& git status --short)
if (-not [string]::IsNullOrWhiteSpace($dirty)) {
  throw "Working tree is not clean. Commit or stash changes before release."
}

Invoke-CheckedGit -Args @("fetch", $Remote, "--tags")
Invoke-CheckedGit -Args @("pull", "--rebase", $Remote, $Branch)

if ([string]::IsNullOrWhiteSpace($Version)) {
  $Version = Get-NextPatchVersion $repoPath
}

if ($Version -notmatch "^\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$") {
  throw "Version '$Version' is not a valid release tag. Expected something like 3.0.18."
}

$existingLocal = (@(& git tag --list $Version) -join "").Trim()
if (-not [string]::IsNullOrWhiteSpace($existingLocal)) {
  throw "Local tag '$Version' already exists."
}

$existingRemote = (@(& git ls-remote --tags $Remote "refs/tags/$Version") -join "").Trim()
if (-not [string]::IsNullOrWhiteSpace($existingRemote)) {
  throw "Remote tag '$Version' already exists."
}

$headSha = (& git rev-parse HEAD).Trim()
Invoke-CheckedGit -Args @("push", $Remote, $Branch)
Invoke-CheckedGit -Args @("tag", "-a", $Version, "-m", "Release $Version")
Invoke-CheckedGit -Args @("push", $Remote, $Version)

Write-Host "Release tag pushed: $Version ($headSha)"

if ($SkipWait) {
  Write-Host "Skipped workflow wait. Check: https://github.com/$repoPath/actions"
  return
}

$run = Wait-GitHubWorkflow $repoPath $Version $headSha $WorkflowName $TimeoutMinutes
$release = Test-GitHubReleaseAssets $repoPath $Version $RequiredAssets

Write-Host "Release completed: $($release.html_url)"
Write-Host "Workflow completed: $($run.html_url)"
