#!/bin/bash
# local-dev.sh — Create a local KinD development environment for the Quay Operator.
#
# Usage:
#   ./hack/local-dev.sh up      Create cluster, deploy dependencies, create QuayRegistry
#   ./hack/local-dev.sh down    Tear down the KinD cluster
#   ./hack/local-dev.sh status  Show environment status
#
# After 'up', start the operator in a separate terminal:
#   SKIP_RESOURCE_REQUESTS=true make run
set -euo pipefail

CLUSTER_NAME="quay-dev"
NAMESPACE="quay"
REGISTRY_NAME="local"
KIND_CONFIG="hack/kind-config.yaml"
CRD_PATH="bundle/manifests/quayregistries.crd.yaml"

# Use podman as kind provider when docker is unavailable.
if ! command -v docker &>/dev/null && command -v podman &>/dev/null; then
  export KIND_EXPERIMENTAL_PROVIDER=podman
fi

check_prerequisites() {
  local missing=()
  for cmd in go kind openssl; do
    command -v "$cmd" &>/dev/null || missing+=("$cmd")
  done
  # Accept either kubectl or oc.
  if ! command -v kubectl &>/dev/null && ! command -v oc &>/dev/null; then
    missing+=("kubectl (or oc)")
  fi
  if ! command -v docker &>/dev/null && ! command -v podman &>/dev/null; then
    missing+=("podman (or docker)")
  fi
  if [ ${#missing[@]} -gt 0 ]; then
    echo "ERROR: missing prerequisites: ${missing[*]}"
    exit 1
  fi

  # Alias oc as kubectl when kubectl is absent.
  if ! command -v kubectl &>/dev/null; then
    shim_dir="$(mktemp -d)"
    ln -s "$(command -v oc)" "${shim_dir}/kubectl"
    export PATH="${shim_dir}:${PATH}"
  fi
}

ensure_podman_running() {
  if [ "${KIND_EXPERIMENTAL_PROVIDER:-}" != "podman" ]; then
    return
  fi
  if ! podman info &>/dev/null 2>&1; then
    echo "Starting Podman machine..."
    podman machine start 2>&1 || true
  fi
  local mem
  mem=$(podman machine inspect 2>/dev/null \
    | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['Resources']['Memory'])" 2>/dev/null || echo 0)
  if [ "${mem}" -lt 8192 ]; then
    echo "WARNING: Podman machine has ${mem} MB RAM. 8192+ MB recommended."
    echo "  Resize with: podman machine stop && podman machine set --memory 16384 && podman machine start"
  fi
}

cluster_exists() {
  kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"
}

cmd_up() {
  check_prerequisites
  ensure_podman_running

  if cluster_exists; then
    echo "KinD cluster '${CLUSTER_NAME}' already exists, reusing it."
  else
    echo "Creating KinD cluster '${CLUSTER_NAME}'..."
    kind create cluster --name "${CLUSTER_NAME}" --config "${KIND_CONFIG}"
  fi

  echo ""
  echo "Installing QuayRegistry CRD..."
  kubectl apply -f "${CRD_PATH}"

  echo ""
  echo "Creating namespace '${NAMESPACE}'..."
  kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

  echo ""
  echo "Setting up Garage S3 storage..."
  bash hack/setup-kind-e2e.sh

  echo ""
  echo "Reading Garage credentials..."
  local access_key secret_key bucket endpoint
  access_key=$(kubectl get configmap garage-creds -n garage-system -o jsonpath='{.data.access-key}')
  secret_key=$(kubectl get configmap garage-creds -n garage-system -o jsonpath='{.data.secret-key}')
  bucket=$(kubectl get configmap garage-creds -n garage-system -o jsonpath='{.data.bucket}')
  endpoint=$(kubectl get configmap garage-creds -n garage-system -o jsonpath='{.data.endpoint}')

  echo "Generating self-signed TLS certificate..."
  local certdir
  certdir="$(mktemp -d)"
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
    -keyout "${certdir}/ssl.key" -out "${certdir}/ssl.cert" -days 30 \
    -subj "/O=Quay Dev" \
    -addext "subjectAltName=IP:127.0.0.1,DNS:localhost" 2>/dev/null

  echo "Creating config bundle secret..."
  kubectl create secret generic "${REGISTRY_NAME}-config-bundle" -n "${NAMESPACE}" \
    --from-literal=config.yaml="$(cat <<EOF
FEATURE_MAILING: false
SERVER_HOSTNAME: "127.0.0.1:30443"
PREFERRED_URL_SCHEME: https
WORKER_COUNT: 1
WORKER_COUNT_WEB: 1
WORKER_COUNT_SECSCAN: 1
WORKER_COUNT_REGISTRY: 1
DB_CONNECTION_POOLING: false
CREATE_NAMESPACE_ON_PUSH: true
FEATURE_USER_INITIALIZE: true
FEATURE_PROXY_STORAGE: true
DISTRIBUTED_STORAGE_CONFIG:
  default:
    - RadosGWStorage
    - access_key: ${access_key}
      secret_key: ${secret_key}
      bucket_name: ${bucket}
      hostname: ${endpoint}
      port: 3900
      is_secure: false
      storage_path: /datastorage/registry
DISTRIBUTED_STORAGE_DEFAULT_LOCATIONS:
  - default
DISTRIBUTED_STORAGE_PREFERENCE:
  - default
EOF
)" \
    --from-file=ssl.cert="${certdir}/ssl.cert" \
    --from-file=ssl.key="${certdir}/ssl.key" \
    --from-file=extra_ca_cert_dev-ca.crt="${certdir}/ssl.cert" \
    --dry-run=client -o yaml | kubectl apply -f -

  rm -rf "${certdir}"

  echo ""
  echo "Creating QuayRegistry '${REGISTRY_NAME}'..."
  cat <<EOF | kubectl apply -f -
apiVersion: quay.redhat.com/v1
kind: QuayRegistry
metadata:
  name: ${REGISTRY_NAME}
  namespace: ${NAMESPACE}
spec:
  configBundleSecret: ${REGISTRY_NAME}-config-bundle
  components:
  - kind: quay
    managed: true
  - kind: postgres
    managed: true
  - kind: redis
    managed: true
  - kind: clair
    managed: true
  - kind: clairpostgres
    managed: true
  - kind: mirror
    managed: true
  - kind: horizontalpodautoscaler
    managed: false
  - kind: objectstorage
    managed: false
  - kind: route
    managed: false
  - kind: monitoring
    managed: false
  - kind: tls
    managed: false
EOF

  echo ""
  echo "Creating NodePort service..."
  cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: ${REGISTRY_NAME}-quay-nodeport
  namespace: ${NAMESPACE}
spec:
  type: NodePort
  selector:
    quay-component: quay-app
    quay-operator/quayregistry: ${REGISTRY_NAME}
  ports:
  - name: https
    port: 8443
    targetPort: 8443
    nodePort: 30443
    protocol: TCP
EOF

  echo ""
  echo "============================================"
  echo " Local development environment is ready!"
  echo "============================================"
  echo ""
  echo "Next steps:"
  echo ""
  echo "  1. Start the operator (in a separate terminal):"
  echo "     SKIP_RESOURCE_REQUESTS=true make run"
  echo ""
  echo "  2. Wait for pods to be ready:"
  echo "     make local-dev-status"
  echo ""
  echo "  3. Initialize the superuser:"
  echo "     curl -sk https://127.0.0.1:30443/api/v1/user/initialize \\"
  echo "       -X POST -H 'Content-Type: application/json' \\"
  echo "       -d '{\"username\":\"admin\",\"password\":\"password123\",\"email\":\"admin@test.com\"}'"
  echo ""
  echo "  4. Push a test image:"
  echo "     crane auth login --insecure 127.0.0.1:30443 -u admin -p password123"
  echo "     crane copy --insecure --platform linux/amd64 \\"
  echo "       quay.io/quay/busybox:latest 127.0.0.1:30443/admin/test:latest"
  echo ""
  echo "  Registry: https://127.0.0.1:30443 (self-signed cert)"
  echo ""
}

cmd_down() {
  if cluster_exists; then
    echo "Deleting KinD cluster '${CLUSTER_NAME}'..."
    kind delete cluster --name "${CLUSTER_NAME}"
    echo "Cluster deleted."
  else
    echo "KinD cluster '${CLUSTER_NAME}' does not exist."
  fi
}

cmd_status() {
  if ! cluster_exists; then
    echo "KinD cluster '${CLUSTER_NAME}' does not exist. Run 'make local-dev-up' to create it."
    exit 1
  fi

  echo "=== Cluster ==="
  kubectl cluster-info --context "kind-${CLUSTER_NAME}" 2>&1 | head -2
  echo ""

  echo "=== Pods (namespace: ${NAMESPACE}) ==="
  kubectl get pods -n "${NAMESPACE}" 2>&1 || echo "(namespace not found)"
  echo ""

  echo "=== QuayRegistry Status ==="
  kubectl get quayregistry "${REGISTRY_NAME}" -n "${NAMESPACE}" \
    -o jsonpath='{range .status.conditions[*]}{.type}: {.status} - {.message}{"\n"}{end}' 2>&1 \
    || echo "(QuayRegistry not found)"
  echo ""
}

case "${1:-}" in
  up)     cmd_up ;;
  down)   cmd_down ;;
  status) cmd_status ;;
  *)
    echo "Usage: $0 {up|down|status}"
    echo ""
    echo "  up      Create KinD cluster and deploy Quay dependencies"
    echo "  down    Tear down the KinD cluster"
    echo "  status  Show environment status"
    exit 1
    ;;
esac
