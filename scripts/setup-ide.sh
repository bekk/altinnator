#!/usr/bin/env bash
# Points GoLand/IntelliJ at the mise-installed Go SDK.
#
# Links .goroot to the Go version selected by mise and registers that Go as the project's GOROOT in
# .idea/workspace.xml. The IDE does not follow the symlink there, so the resolved path is written.
# .idea/ is not committed, so this is done by script.
#
# The IDE only reads GOROOT from workspace.xml when the project is opened, so an already open IDE
# needs a one-time manual selection; the instruction is printed below.
set -euo pipefail

cd "$(dirname "$0")/.."

goroot="$(mise where go)"
ln -sfn "$goroot" .goroot

workspace=.idea/workspace.xml
# Use the IDE's $USER_HOME$ macro for paths under the home directory, like the IDE does itself.
case "$goroot" in
  "$HOME"/*) goroot_url="file://\$USER_HOME\$${goroot#"$HOME"}" ;;
  *) goroot_url="file://${goroot}" ;;
esac
export GOROOT_COMPONENT="<component name=\"GOROOT\" url=\"${goroot_url}\" />"

if grep -qF "$GOROOT_COMPONENT" "$workspace" 2>/dev/null; then
  :
elif [ ! -f "$workspace" ]; then
  mkdir -p .idea
  printf '<?xml version="1.0" encoding="UTF-8"?>\n<project version="4">\n  %s\n</project>\n' "$GOROOT_COMPONENT" >"$workspace"
elif grep -q '<component name="GOROOT"' "$workspace"; then
  perl -0pi -e 's|<component name="GOROOT"[^>]*/>|$ENV{GOROOT_COMPONENT}|' "$workspace"
else
  perl -0pi -e 's|</project>|  $ENV{GOROOT_COMPONENT}\n</project>|' "$workspace"
fi

echo "✅  IDE GOROOT set to ${goroot}"
echo "👉  If the project is already open in GoLand, do this once: click \"Set up Go\" in the"
echo "    \"Go SDK is not specified for this project\" banner (or Settings → Go → GOROOT) and select ${goroot}"
