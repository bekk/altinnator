#!/usr/bin/env bash
# Makes sure mise is available (offering to install it with the official installer) and the project's pinned tools are installed
# (the postinstall hook in mise.toml also links mise's Go into the IDE), then runs mise with the given arguments.
#
# Usage: scripts/bootstrap.sh run <task> [args...]
set -euo pipefail

cd "$(dirname "$0")/.."

find_mise() {
  command -v mise 2>/dev/null && return 0
  local candidate
  for candidate in "$HOME/.local/bin/mise" /opt/homebrew/bin/mise /usr/local/bin/mise; do
    if [ -x "$candidate" ]; then
      echo "$candidate"
      return 0
    fi
  done
  return 1
}

if ! mise_bin="$(find_mise)"; then
  echo "mise is not installed. This project uses it to provide Go and all development tools."
  echo "See https://mise.jdx.dev/getting-started.html"

  if [ ! -t 0 ]; then
    echo "❌  Not an interactive terminal, so cannot ask to install it. Install mise and run this again."
    exit 1
  fi

  read -r -p "Install it now with 'curl -fsSL https://mise.run | sh'? [y/N] " answer
  case "$answer" in
    y | Y | yes | YES) ;;
    *)
      echo "❌  mise is required. Install it and run this again."
      exit 1
      ;;
  esac

  curl -fsSL https://mise.run | sh

  mise_bin="$(find_mise)" || {
    echo "❌  mise was installed but could not be found. Open a new terminal and run this again."
    exit 1
  }
  echo "✅  mise installed. To use it in your terminal, activate it in your shell: https://mise.jdx.dev/getting-started.html#activate-mise"
fi

# Running this script inside the repository is taken as trusting its mise.toml.
"$mise_bin" trust --quiet
echo "Installing the project's tools. The first time this can take a few minutes, as some tools are compiled from source."
"$mise_bin" install

exec "$mise_bin" "$@"
