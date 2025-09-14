#!/usr/bin/env bash
set -euo pipefail

# Release helper script
# - Computes next semver (vMAJOR.MINOR.PATCH) based on Conventional Commits since last v* tag
# - Generates Helm chart with that version
# - Commits chart changes
# - Creates an annotated git tag
#
# Usage:
#   scripts/release.sh [--dry-run] [--no-commit] [--push]
# Env (optional):
#   GHCR_REPO=ghcr.io/sladg/datarestor-operator
#   CHART_NAME=datarestor-operator
#   CHART_OUT_DIR=charts/datarestor-operator
#
# Notes:
# - Requires: git, kustomize, helmify, yq (same expectations as gen-helm.sh)

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT_DIR"

DRY_RUN=false
PUSH=false
NO_COMMIT=false

for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=true ;;
    --push) PUSH=true ;;
    --no-commit) NO_COMMIT=true ;;
    *) echo "Unknown arg: $arg" >&2; exit 1 ;;
  esac
done

GHCR_REPO="${GHCR_REPO:-ghcr.io/sladg/datarestor-operator}"
CHART_NAME="${CHART_NAME:-datarestor-operator}"
CHART_OUT_DIR="${CHART_OUT_DIR:-charts/$CHART_NAME}"

# Ensure working tree is clean unless dry-run or no-commit
if ! $DRY_RUN && ! $NO_COMMIT; then
  if [ -n "$(git status --porcelain)" ]; then
    echo "Error: Working tree is dirty. Please commit or stash changes before releasing." >&2
    exit 1
  fi
fi

# Determine last tag (v*) and commits since
LAST_TAG=$(git describe --tags --abbrev=0 --match 'v*' 2>/dev/null || echo "v0.0.0")
COMMITS=$(git log --pretty=format:'%s%n%b' "${LAST_TAG}..HEAD" || true)

if [ -z "$COMMITS" ]; then
  echo "No commits since $LAST_TAG. Nothing to release."
  exit 0
fi

# Determine bump level: major if BREAKING CHANGE or !, minor if feat, patch otherwise
BUMP="patch"
if echo "$COMMITS" | grep -Eq "(^|\n)([^\n]*!)|BREAKING CHANGE"; then
  BUMP="major"
elif echo "$COMMITS" | grep -Eqi "(^|\n)feat(\(|:)|^feat!"; then
  BUMP="minor"
else
  # If there's any commits and no feat/breaking, patch
  BUMP="patch"
fi

# Parse last version numbers
ver_no_v=${LAST_TAG#v}
MAJOR=${ver_no_v%%.*}
rest=${ver_no_v#*.}
MINOR=${rest%%.*}
PATCH=${rest#*.}

inc() { echo "$(( $1 + 1 ))"; }

case "$BUMP" in
  major)
    MAJOR=$(inc "$MAJOR"); MINOR=0; PATCH=0 ;;
  minor)
    MINOR=$(inc "$MINOR"); PATCH=0 ;;
  patch)
    PATCH=$(inc "$PATCH") ;;
esac

NEXT_TAG="v${MAJOR}.${MINOR}.${PATCH}"

echo "Last tag:    $LAST_TAG"
echo "Bump level:  $BUMP"
echo "Next tag:    $NEXT_TAG"

# Generate Helm chart with NEXT_TAG as app version (and chart version without v)
if $DRY_RUN; then
  echo "[DRY-RUN] Would generate chart with APP_VERSION=$NEXT_TAG"
else
  GHCR_REPO="$GHCR_REPO" APP_VERSION="$NEXT_TAG" CHART_NAME="$CHART_NAME" CHART_OUT_DIR="$CHART_OUT_DIR" \
    "$ROOT_DIR/scripts/gen-helm.sh"
fi

# Commit and tag
if $NO_COMMIT; then
  echo "Skipping commit/tag per --no-commit"
  exit 0
fi

if $DRY_RUN; then
  echo "[DRY-RUN] Would commit and tag $NEXT_TAG"
else
  git add "$CHART_OUT_DIR"
  git commit -m "chore(release): $NEXT_TAG"
  git tag -a "$NEXT_TAG" -m "$NEXT_TAG"
  echo "Created tag $NEXT_TAG"
  if $PUSH; then
    git push --follow-tags && git push --tags
    echo "Pushed tag and commits."
  else
    echo "Run 'git push --follow-tags && git push --tags' to publish the release."
  fi
fi