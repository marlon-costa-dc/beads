#!/usr/bin/env bash
set -euo pipefail

distribution_paths=(
    .github/workflows/release.yml
    .goreleaser.yml
    install.ps1
    npm-package/package.json
    npm-package/scripts/postinstall.js
    scripts/install.sh
)

if hits=$(git grep -n -E 'gastownhall/beads|steveyegge/beads|go install .*beads.*/cmd/bd@latest' -- "${distribution_paths[@]}"); then
    printf '%s\n' "$hits" >&2
    printf 'error: DC distribution references an upstream or floating install source\n' >&2
    exit 1
fi
