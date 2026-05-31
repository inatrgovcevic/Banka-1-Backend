Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
Push-Location $root
try {
  docker run --rm `
    -v "${root}:/workspace" `
    -w /workspace `
    bufbuild/buf:1.46.0 `
    generate
}
finally {
  Pop-Location
}
