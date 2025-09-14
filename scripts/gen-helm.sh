#!/usr/bin/env bash
set -euo pipefail

# Script to generate the Helm chart using helmify with proper image and version stamping.
# Usage (env-driven):
#   GHCR_REPO=ghcr.io/your-org/your-image \
#   APP_VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo dev) \
#   CHART_NAME=datarestor-operator \
#   CHART_OUT_DIR=charts/datarestor-operator \
#   ./scripts/gen-helm.sh
#
# Notes:
# - Works entirely in dist/ as staging, then syncs to CHART_OUT_DIR.
# - Requires: kustomize, helmify, yq available on PATH or in ./bin/

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# Defaults
GHCR_REPO="${GHCR_REPO:-ghcr.io/sladg/datarestor-operator}"
APP_VERSION="${APP_VERSION:-$(git -C "$ROOT_DIR" describe --tags --match 'v*' --always --dirty 2>/dev/null || echo dev)}"
CHART_NAME="${CHART_NAME:-datarestor-operator}"
CHART_OUT_DIR_REL="${CHART_OUT_DIR:-charts/${CHART_NAME}}"
CHART_OUT_DIR="$ROOT_DIR/$CHART_OUT_DIR_REL"
KUSTOMIZE_DIR="$ROOT_DIR/config/default"
STAGE_DIR="$ROOT_DIR/dist/helmify_stage"
DIST_DIR="$ROOT_DIR/dist"

find_tool() {
  local name="$1"
  if [ -x "$ROOT_DIR/bin/$name" ]; then
    echo "$ROOT_DIR/bin/$name"
    return 0
  fi
  if command -v "$name" >/dev/null 2>&1; then
    command -v "$name"
    return 0
  fi
  echo "Error: $name not found. Please install it or place it in $ROOT_DIR/bin/$name" >&2
  exit 1
}

KUSTOMIZE_BIN=$(find_tool kustomize)
HELMIFY_BIN=$(find_tool helmify)
YQ_BIN=$(find_tool yq)

# Prepare staging
rm -rf "$STAGE_DIR" "$DIST_DIR/$CHART_NAME"
mkdir -p "$DIST_DIR" "$STAGE_DIR"
cp -R "$ROOT_DIR/config" "$STAGE_DIR/"

# Set release image in the staged manager kustomization
(
  cd "$STAGE_DIR/config/manager"
  "$KUSTOMIZE_BIN" edit set image "controller=${GHCR_REPO}:${APP_VERSION}"
)

# Generate Helm chart into dist using helmify
(
  cd "$DIST_DIR"
  "$KUSTOMIZE_BIN" build "$STAGE_DIR/config/default" | "$HELMIFY_BIN" "$CHART_NAME"
)

CHART_DIR="$DIST_DIR/$CHART_NAME"
if [ ! -f "$CHART_DIR/Chart.yaml" ]; then
  echo "Error: Chart.yaml not found at $CHART_DIR/Chart.yaml. helmify may have failed." >&2
  exit 1
fi

# Stamp chart version and appVersion (strip leading 'v' for chart version)
CHART_VERSION="${APP_VERSION#v}"
"$YQ_BIN" -i ".version = \"$CHART_VERSION\" | .appVersion = \"$APP_VERSION\"" "$CHART_DIR/Chart.yaml"

# Update values.yaml with the correct image repository and tag
if [ -f "$CHART_DIR/values.yaml" ]; then
"$YQ_BIN" -i ".controllerManager.manager.image.repository = \"$GHCR_REPO\" | .controllerManager.manager.image.tag = \"$CHART_VERSION\"" "$CHART_DIR/values.yaml"
else
  echo "Warning: values.yaml not found in $CHART_DIR. Skipping values stamping." >&2
fi

# Sync chart into target output directory under charts/
rm -rf "$CHART_OUT_DIR"
mkdir -p "$(dirname "$CHART_OUT_DIR")"
cp -R "$CHART_DIR" "$CHART_OUT_DIR"

echo "Helm chart generated and stamped:"
echo "  Chart dir: $CHART_OUT_DIR"
echo "  Chart version: $CHART_VERSION"
echo "  App version: $APP_VERSION"
echo "  Image: ${GHCR_REPO}:${CHART_VERSION}"
