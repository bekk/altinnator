#!/bin/bash
set -eo pipefail

KUBECONTEXT=${KUBECONTEXT:-"kind-altinnator"}
CHAINSAW_BIN="${CHAINSAW_BIN:-chainsaw}"

"${CHAINSAW_BIN}" test --kube-context "$KUBECONTEXT" --config test/chainsaw/config.yaml --test-dir test/chainsaw && echo "✅ Tests succeeded" || (echo "❌ Tests failed" && exit 1)