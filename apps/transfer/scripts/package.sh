#!/usr/bin/env bash
# Local cross-builds only: no upload, publishing, deployment or code signing.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
if [[ $# -lt 1 || $# -gt 2 || ! "$1" =~ ^v?[0-9][0-9A-Za-z.+-]*$ ]]; then
  echo "Usage: $0 VERSION [NEW_OUTPUT_DIRECTORY]" >&2
  exit 2
fi
version="$1"
output="${2:-$root/bin/alpha/$version}"
mkdir -p "$(dirname "$output")"
# Never overwrite a previous package set (including its checksums).
mkdir "$output"
output="$(cd "$output" && pwd)"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
if command -v sha256sum >/dev/null; then
  checksum=(sha256sum)
else
  checksum=(shasum -a 256)
fi
revision="$(git rev-parse HEAD)"
if [[ -n "$(git status --porcelain --untracked-files=normal)" ]]; then
  revision="$revision (working tree modified)"
fi

# Explicit alpha build matrix, not a claim of native runtime validation.
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  name="transfer_${version}_${os}_${arch}"
  mkdir "$stage/$name"
  echo "Building $target..."
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -o "$stage/$name/transfer" ./cmd/transfer
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath -o "$stage/$name/transfer-mcp" ./cmd/transfer-mcp
  cp docs/alpha.md "$stage/$name/ALPHA.md"
  cp docs/mcp.md "$stage/$name/MCP.md"
  printf 'Transfer %s\nTarget: %s\nSource: %s\nToolchain: %s\n' \
    "$version" "$target" "$revision" "$(go version)" > "$stage/$name/BUILD.txt"
  tar -czf "$output/$name.tar.gz" -C "$stage" "$name"
done
(
  cd "$output"
  "${checksum[@]}" ./*.tar.gz > SHA256SUMS
)
echo "Local alpha archives and SHA256SUMS: $output"
