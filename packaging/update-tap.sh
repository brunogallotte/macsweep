#!/usr/bin/env bash
# Regenerates the Homebrew formula in the tap repository for a released
# version. Run from CI after goreleaser has published the release.
set -euo pipefail

version="${1:?uso: update-tap.sh <versao sem o v>}"
repo="brunogallotte/macsweep"
tap="brunogallotte/homebrew-tap"
asset="macsweep_${version}_darwin_universal.tar.gz"
url="https://github.com/${repo}/releases/download/v${version}/${asset}"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

curl -fsSL "$url" -o "$tmp/$asset"
sha="$(shasum -a 256 "$tmp/$asset" | cut -d' ' -f1)"

sed -e "s/__VERSION__/${version}/g" -e "s/__SHA256__/${sha}/g" \
  packaging/macsweep.rb.tmpl > "$tmp/macsweep.rb"

# In CI a token is injected. Run locally, the user's own git credentials
# (or gh) handle the push, so no token is needed.
if [ -n "${TAP_TOKEN:-}" ]; then
  clone_url="https://x-access-token:${TAP_TOKEN}@github.com/${tap}.git"
else
  clone_url="https://github.com/${tap}.git"
fi
git clone --depth 1 "$clone_url" "$tmp/tap"
mkdir -p "$tmp/tap/Formula"
cp "$tmp/macsweep.rb" "$tmp/tap/Formula/macsweep.rb"

cd "$tmp/tap"
git config user.name brunogallotte
git config user.email origo.hq@gmail.com
git add Formula/macsweep.rb
git commit -m "macsweep ${version}"
git push

echo "fórmula publicada: brew install ${tap%/*}/tap/macsweep"
