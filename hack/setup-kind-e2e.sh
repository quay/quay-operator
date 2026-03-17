#!/bin/bash
# setup-kind-e2e.sh — Deploy Garage S3 storage on a KinD cluster for e2e tests.
# Creates a ConfigMap with S3 credentials that chainsaw test scripts read.
set -euo pipefail

GARAGE_NS="garage-system"
GARAGE_IMAGE="dxflrs/garage:v1.0.1"
GARAGE_RPC_SECRET="5c61c8739e9c6a1046347438ca7a1c77fc3ea1af4b0ef21573a1a11c8e8f4a08"
GARAGE_ADMIN_TOKEN="admin-token"
BUCKET_NAME="quay-datastore"

echo "=== Setting up Garage S3 for KinD e2e ==="

# Create namespace
kubectl create namespace "${GARAGE_NS}" --dry-run=client -o yaml | kubectl apply -f -

# Deploy Garage configmap
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: garage-config
  namespace: ${GARAGE_NS}
data:
  garage.toml: |
    replication_factor = 1
    metadata_dir = "/tmp/meta"
    data_dir = "/tmp/data"
    db_engine = "sqlite"
    rpc_bind_addr = "[::]:3901"
    rpc_secret = "${GARAGE_RPC_SECRET}"

    [s3_api]
    api_bind_addr = "[::]:3900"
    s3_region = "us-east-1"
    root_domain = ".s3.garage.localhost"

    [s3_web]
    bind_addr = "[::]:3902"
    root_domain = ".web.garage.localhost"

    [admin]
    api_bind_addr = "[::]:3903"
    admin_token = "${GARAGE_ADMIN_TOKEN}"
EOF

# Deploy Garage
cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: garage
  namespace: ${GARAGE_NS}
  labels:
    app: garage
spec:
  replicas: 1
  selector:
    matchLabels:
      app: garage
  template:
    metadata:
      labels:
        app: garage
    spec:
      containers:
      - name: garage
        image: ${GARAGE_IMAGE}
        ports:
        - name: s3
          containerPort: 3900
        - name: rpc
          containerPort: 3901
        - name: admin
          containerPort: 3903
        volumeMounts:
        - name: config
          mountPath: /etc/garage.toml
          subPath: garage.toml
        readinessProbe:
          httpGet:
            path: /health
            port: 3903
            httpHeaders:
            - name: Authorization
              value: "Bearer ${GARAGE_ADMIN_TOKEN}"
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: garage-config
---
apiVersion: v1
kind: Service
metadata:
  name: garage
  namespace: ${GARAGE_NS}
  labels:
    app: garage
spec:
  selector:
    app: garage
  ports:
  - name: s3
    port: 3900
    targetPort: 3900
  - name: admin
    port: 3903
    targetPort: 3903
EOF

# Wait for Garage to be ready
echo "Waiting for Garage deployment..."
kubectl rollout status deployment/garage -n "${GARAGE_NS}" --timeout=120s

# Find the garage pod
GARAGE_POD=$(kubectl get pods -n "${GARAGE_NS}" -l app=garage -o jsonpath='{.items[0].metadata.name}')
echo "Garage pod: ${GARAGE_POD}"

garage_exec() {
  kubectl exec -n "${GARAGE_NS}" "${GARAGE_POD}" -- /garage "$@" 2>&1
}

# Get node ID
NODE_ID=$(garage_exec node id -q | tr -d '[:space:]')
echo "Node ID: ${NODE_ID}"

# Assign layout (idempotent)
garage_exec layout assign -z dc1 -c 1G "${NODE_ID}" || true

# Apply layout (try versions 1-5 for idempotency)
APPLIED=false
for v in 1 2 3 4 5; do
  if garage_exec layout apply --version "$v" 2>/dev/null; then
    echo "Layout applied (version $v)"
    APPLIED=true
    break
  fi
done
if [ "${APPLIED}" = "false" ]; then
  echo "Layout may already be applied"
fi

# Create bucket (idempotent)
garage_exec bucket create "${BUCKET_NAME}" || true

# Create access key
KEY_NAME="quay-e2e-$(date +%s)"
KEY_OUTPUT=$(garage_exec key create "${KEY_NAME}")
echo "Key output: ${KEY_OUTPUT}"

# Parse key ID and secret from output
ACCESS_KEY=$(echo "${KEY_OUTPUT}" | grep -oP 'Key ID: \K\S+' || echo "${KEY_OUTPUT}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('accessKeyId',''))" 2>/dev/null || true)
SECRET_KEY=$(echo "${KEY_OUTPUT}" | grep -oP 'Secret key: \K\S+' || echo "${KEY_OUTPUT}" | python3 -c "import sys,json; print(json.load(sys.stdin).get('secretAccessKey',''))" 2>/dev/null || true)

if [ -z "${ACCESS_KEY}" ] || [ -z "${SECRET_KEY}" ]; then
  echo "ERROR: Failed to parse access key from output"
  echo "${KEY_OUTPUT}"
  exit 1
fi

echo "Access Key: ${ACCESS_KEY}"

# Grant bucket permissions
garage_exec bucket allow --read --write --owner "${BUCKET_NAME}" --key "${ACCESS_KEY}"

# Store credentials in a ConfigMap for chainsaw tests to read
kubectl create configmap garage-creds -n "${GARAGE_NS}" \
  --from-literal=access-key="${ACCESS_KEY}" \
  --from-literal=secret-key="${SECRET_KEY}" \
  --from-literal=bucket="${BUCKET_NAME}" \
  --from-literal=endpoint="garage.${GARAGE_NS}.svc.cluster.local" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "=== Garage S3 setup complete ==="
echo "Credentials stored in configmap/${GARAGE_NS}/garage-creds"
