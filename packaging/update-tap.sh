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

git clone --depth 1 "https://x-access-token:${TAP_TOKEN}@github.com/${tap}.git" "$tmp/tap"
mkdir -p "$tmp/tap/Formula"
cp "$tmp/macsweep.rb" "$tmp/tap/Formula/macsweep.rb"

cd "$tmp/tap"
git config user.name brunogallotte
git config user.email origo.hq@gmail.com
git add Formula/macsweep.rb
git commit -m "macsweep ${version}"
git push
