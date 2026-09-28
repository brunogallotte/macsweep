#!/usr/bin/env sh
# Instala o macsweep sem Homebrew.
#   curl -fsSL https://raw.githubusercontent.com/brunogallotte/macsweep/main/install.sh | sh
set -eu

repo="brunogallotte/macsweep"
prefix="${MACSWEEP_PREFIX:-/usr/local/bin}"

[ "$(uname -s)" = "Darwin" ] || { echo "macsweep so roda em macOS"; exit 1; }

version="$(curl -fsSL "https://api.github.com/repos/${repo}/releases/latest" |
  sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -1)"
[ -n "$version" ] || { echo "nao consegui descobrir a ultima versao"; exit 1; }

asset="macsweep_${version}_darwin_universal.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "baixando macsweep ${version}..."
curl -fsSL "https://github.com/${repo}/releases/download/v${version}/${asset}" -o "$tmp/$asset"
tar -xzf "$tmp/$asset" -C "$tmp"

if [ -w "$prefix" ]; then
  install -m 0755 "$tmp/macsweep" "$prefix/macsweep"
else
  echo "escrevendo em ${prefix}, pode pedir sua senha"
  sudo install -m 0755 "$tmp/macsweep" "$prefix/macsweep"
fi

echo "pronto: $("$prefix/macsweep" --version)"
