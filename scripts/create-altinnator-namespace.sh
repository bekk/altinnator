#!/bin/bash
set -eo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-altinnator"}
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"

echo "🤞  Creating namespace: altinnator-system"

# Attempt to create the namespace and capture both stdout and stderr
# NOTE: `set -e` would abort the script on a non-zero exit code here (e.g. AlreadyExists),
# so we temporarily disable it to handle the error explicitly.
set +e
output=$("${KUBECTL_BIN}" create namespace "altinnator-system" --context "$KUBECONTEXT" 2>&1)
exit_code=$?
set -e

# Check the exit code and output
if [ $exit_code -eq 0 ]; then
    echo "✅  Namespace 'altinnator-system' created successfully"
elif echo "$output" | grep -qiE "already exists|AlreadyExists"; then
    echo "✅  Namespace 'altinnator-system' already exists, continuing..."
else
    echo -e "❌  Error creating 'altinnator-system' namespace:"
    echo "$output"
    exit 1
fi