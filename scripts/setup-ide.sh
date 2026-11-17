#!/usr/bin/env bash
# Points GoLand/IntelliJ at the mise-installed Go SDK.
#
# Links .goroot to the Go version selected by mise and registers it as the project's GOROOT in
# .idea/workspace.xml. .idea/ is not committed, so this is done by script. A running IDE rewrites
# workspace.xml from memory, so run this before opening the project (or restart the IDE afterwards).
set -euo pipefail

cd "$(dirname "$0")/.."

goroot="$(mise where go)"
ln -sfn "$goroot" .goroot

workspace=.idea/workspace.xml
export GOROOT_COMPONENT='<component name="GOROOT" url="file://$PROJECT_DIR$/.goroot" />'

if [ ! -f "$workspace" ]; then
  mkdir -p .idea
  printf '<?xml version="1.0" encoding="UTF-8"?>\n<project version="4">\n  %s\n</project>\n' "$GOROOT_COMPONENT" >"$workspace"
elif grep -q '<component name="GOROOT"' "$workspace"; then
  perl -0pi -e 's|<component name="GOROOT"[^>]*/>|$ENV{GOROOT_COMPONENT}|' "$workspace"
else
  perl -0pi -e 's|</project>|  $ENV{GOROOT_COMPONENT}\n</project>|' "$workspace"
fi

echo "✅  IDE GOROOT set to .goroot -> ${goroot}"
