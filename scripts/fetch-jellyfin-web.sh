#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
repo_url="${JELLYFIN_WEB_REPO:-https://github.com/jellyfin/jellyfin-web.git}"
output_root="${1:-${JELLYFIN_WEB_OUTPUT_DIR:-$repo_root/third_party/jellyfin-web}}"

default_version="$(
  awk -F'"' '/^const DefaultJellyfinWebVersion = "/ { print $2; exit }' \
    "$repo_root/internal/config/config.go"
)"

if [[ -z "${default_version}" ]]; then
  default_version="10.11.6"
fi

version="${JELLYFIN_WEB_VERSION:-$default_version}"
tag="v${version}"
release_dir="${output_root}/${version}"
tmpdir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required tool: %s\n' "$1" >&2
    exit 1
  fi
}

require_tool git
require_tool npm

if ! command -v node >/dev/null 2>&1; then
  printf 'missing required tool: node\n' >&2
  exit 1
fi

mkdir -p "$output_root"

if ! git ls-remote --exit-code --tags "$repo_url" "refs/tags/${tag}" >/dev/null 2>&1; then
  printf 'upstream tag %s was not found in %s\n' "$tag" "$repo_url" >&2
  printf 'set JELLYFIN_WEB_VERSION to a published release tag and try again\n' >&2
  exit 1
fi

printf 'cloning %s at %s\n' "$repo_url" "$tag"
git clone --depth 1 --branch "$tag" "$repo_url" "$tmpdir/src" >/dev/null

printf 'installing dependencies\n'
(cd "$tmpdir/src" && npm ci >/dev/null)

printf 'building production bundle\n'
(cd "$tmpdir/src" && npm run build:production >/dev/null)

rm -rf "$release_dir"
mkdir -p "$release_dir"
cp -R "$tmpdir/src/dist/." "$release_dir/"

ln -sfn "$version" "$output_root/current"

printf 'bundle written to %s\n' "$release_dir"
printf 'web_dir should point at %s\n' "$output_root/current"
