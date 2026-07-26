#!/usr/bin/env bash
set -euo pipefail

INSTALL_DIR="$HOME/tsx-evaluator"
QUADLET_DIR="$HOME/.config/containers/systemd"
ENV_DIR="$HOME/.config/tsx-evaluator"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "==> Installing project to $INSTALL_DIR"
rm -rf "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR"
cp -r "$PROJECT_DIR"/Containerfile "$PROJECT_DIR"/go.mod "$PROJECT_DIR"/go.sum \
      "$PROJECT_DIR"/cmd "$PROJECT_DIR"/internal "$PROJECT_DIR"/proto "$INSTALL_DIR/"
# Remove local bin/ and gen/ artifacts if present -- the build regenerates them.
rm -rf "$INSTALL_DIR/bin" "$INSTALL_DIR/gen"

# Copy tracker source for the Containerfile build (go.mod replace directive).
# Only bin/ is stripped; gen/ is needed for the evaluator's protobuf imports.
TRACKER_SRC="${TSX_TRACKER_DIR:-}"
if [[ -z "$TRACKER_SRC" && -d /opt/tsx-tracker ]]; then
  TRACKER_SRC="/opt/tsx-tracker"
fi
if [[ -z "$TRACKER_SRC" && -d "$PROJECT_DIR/../tsx-tracker" ]]; then
  TRACKER_SRC="$PROJECT_DIR/../tsx-tracker"
fi
if [[ -z "$TRACKER_SRC" ]]; then
  echo "ERROR: tsx-tracker source not found." >&2
  echo "  Set TSX_TRACKER_DIR=/path/to/tsx-tracker or place it at /opt/tsx-tracker" >&2
  exit 1
fi
echo "==> Copying tracker from $TRACKER_SRC"
rm -rf "$HOME/tsx-tracker"
cp -r "$TRACKER_SRC" "$HOME/tsx-tracker"
rm -rf "$HOME/tsx-tracker/bin"

echo "==> Installing Quadlet units to $QUADLET_DIR"
mkdir -p "$QUADLET_DIR"
cp "$SCRIPT_DIR"/quadlet/tsx-evaluator.build "$QUADLET_DIR/"
cp "$SCRIPT_DIR"/quadlet/tsx-evaluator.container "$QUADLET_DIR/"

echo "==> Installing environment file to $ENV_DIR"
mkdir -p "$ENV_DIR"
cp -n "$PROJECT_DIR"/.env.podman "$ENV_DIR/.env.podman" 2>/dev/null || true
chmod 600 "$ENV_DIR/.env.podman"

echo "==> Reloading systemd"
systemctl --user daemon-reload

echo ""
echo "Done. Next steps:"
echo ""
echo "  1. Edit $ENV_DIR/.env.podman and fill in DB_USER, DB_PASSWORD"
echo ""
echo "  2. Build the image:"
echo "       systemctl --user start tsx-evaluator-build.service"
echo ""
echo "  3. Enable and start the service:"
echo "       systemctl --user enable --now tsx-evaluator.service"
echo ""
echo "  4. Check status:"
echo "       systemctl --user status tsx-evaluator"
echo "       podman logs tsx-evaluator"
