#!/bin/bash
set -eo pipefail

# Allow overriding via env, but default to the defaults in mise.toml
KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME:-altinnator}"
KUBECONTEXT="${KUBECONTEXT:-kind-${KIND_CLUSTER_NAME}}"
KIND_BIN="${KIND_BIN:-kind}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"

# tools
command -v "${KIND_BIN}" >/dev/null 2>&1 || {
  echo "❌  kind is not installed (needed to verify the cluster)." >&2
  exit 1
}
command -v "${KUBECTL_BIN}" >/dev/null 2>&1 || {
  echo "❌  kubectl is not installed." >&2
  exit 1
}

# current context
ctx="$("${KUBECTL_BIN}" config current-context 2>/dev/null || true)"
if [[ "${ctx}" != "${KUBECONTEXT}" ]]; then
  echo "❌  Current kubecontext is '${ctx}' (expected '${KUBECONTEXT}')." >&2
  if "${KIND_BIN}" get clusters 2>/dev/null | grep -qx "${KIND_CLUSTER_NAME}"; then
    echo "    The kind cluster exists. Switch with: kubectx ${KUBECONTEXT}" >&2
  else
    echo "    The kind cluster '${KIND_CLUSTER_NAME}' does not exist. Set it up with: mise run cluster:local" >&2
  fi
  exit 1
fi

echo "✅  Local cluster looks healthy (cluster=${KIND_CLUSTER_NAME}, context=${KUBECONTEXT})."