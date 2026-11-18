#!/usr/bin/env bash
set -euo pipefail

# Allow overriding via env, but default to the defaults in mise.toml
KUBECONTEXT="${KUBECONTEXT:-kind-altinnator}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"

NAMESPACE="altinnator-system"
DEPLOYMENT="altinnator"

check_deployment() {
  echo "🔎 Checking Deployment ${NAMESPACE}/${DEPLOYMENT}..."

  ${KUBECTL_BIN} --context "$KUBECONTEXT" get deployment -n "$NAMESPACE" "$DEPLOYMENT" >/dev/null

  READY=$(${KUBECTL_BIN} --context "$KUBECONTEXT" get deployment -n "$NAMESPACE" "$DEPLOYMENT" \
    -o jsonpath='{.status.readyReplicas}')

  DESIRED=$(${KUBECTL_BIN} --context "$KUBECONTEXT" get deployment -n "$NAMESPACE" "$DEPLOYMENT" \
    -o jsonpath='{.spec.replicas}')

  if [[ "${READY:-0}" != "$DESIRED" ]]; then
    echo "❌ Deployment not ready (ready=${READY:-0}, desired=${DESIRED})"
    exit 1
  fi

  echo "✅ Deployment is healthy (replicas=${DESIRED})."
}

# ---- execution ----

check_deployment

echo "🎉 Altinnator is deployed"