# Windows amd64 candidate only; this script never tags or publishes a Release.
param([Parameter(Mandatory)][ValidatePattern('^v0\.\d+\.\d+-demo\.\d+$')][string]$Version)
$ErrorActionPreference = 'Stop'
if ([Environment]::OSVersion.Platform -ne 'Win32NT') { throw 'Build this package on Windows.' }
$root = Split-Path $PSScriptRoot -Parent
$output = Join-Path $root ('dist/package-' + [guid]::NewGuid().ToString('N'))
$name = "agent-platform-$Version-windows-amd64"
$package = Join-Path $output $name
$previous = @{}
foreach ($key in @('GOOS', 'GOARCH', 'CGO_ENABLED')) { $previous[$key] = [Environment]::GetEnvironmentVariable($key, 'Process') }
Push-Location $root
try {
    New-Item -ItemType Directory -Path "$package/web", "$package/configs" -Force | Out-Null
    $env:GOOS = 'windows'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
    & go build -p=2 -trimpath -o "$package/server.exe" ./cmd/server
    if ($LASTEXITCODE -ne 0) { throw 'Go build failed.' }
    & npm.cmd --prefix web run build | Out-Host
    if ($LASTEXITCODE -ne 0) { throw 'Web build failed.' }
    # Allowlist: never copy local config, database, logs, or repository contents.
    Copy-Item -LiteralPath "$root/web/dist" -Destination "$package/web/dist" -Recurse
    Copy-Item -LiteralPath "$PSScriptRoot/demo-README.md" -Destination "$package/README.md"
    @'
@echo off
cd /d "%~dp0"
echo Open http://127.0.0.1:8080 in your browser. Ctrl+C stops the demo.
server.exe -demo -listen 127.0.0.1:8080 -db data/trips.db -web-dir web/dist
pause
'@ | Set-Content -LiteralPath "$package/start-demo.cmd" -Encoding ascii
    @'
# Optional real-mode example only; real service integration is not certified.
# Never put credentials into a distributed package. Use AGENT_LLM_API_KEY.
llm:
  model: deepseek-chat
  base_url: https://api.deepseek.com/v1
memory:
  max_messages: 20
'@ | Set-Content -LiteralPath "$package/configs/config.example.yaml" -Encoding ascii
    $commit = & git -C $root rev-parse HEAD
    if ($LASTEXITCODE -ne 0) { throw 'Cannot identify source commit.' }
    $dirty = & git -C $root status --porcelain
    if ($LASTEXITCODE -ne 0) { throw 'Cannot identify source status.' }
    @{version=$Version; source_commit=$commit; working_tree_dirty=[bool]$dirty; mode='demo'; target='windows-amd64'} |
        ConvertTo-Json | Set-Content -LiteralPath "$package/build-info.json" -Encoding utf8
    $zip = Join-Path $output "$name.zip"
    Compress-Archive -LiteralPath $package -DestinationPath $zip
    $hash = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToLowerInvariant()
    "$hash  $name.zip" | Set-Content -LiteralPath "$output/SHA256SUMS.txt" -Encoding ascii
    [pscustomobject]@{Archive=$zip; Checksums="$output/SHA256SUMS.txt"; Name=$name}
} finally {
    foreach ($key in $previous.Keys) { [Environment]::SetEnvironmentVariable($key, $previous[$key], 'Process') }
    Pop-Location
}
