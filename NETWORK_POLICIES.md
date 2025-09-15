# Quay Operator Network Policies

This document describes the NetworkPolicy resources generated for the Quay Operator and its components based on the existing YAML configurations.

## Overview

The NetworkPolicies implement a zero-trust network security model for the Quay container registry platform, ensuring that only necessary network communication is allowed between components.

## Components and Network Policies

### 1. Quay App (Main Application)
**File**: `bundle/manifests/quay-network-policies.yaml` - `quay-app-netpol`

**Ports**:
- 80 (HTTP)
- 443 (HTTPS) 
- 8081 (JWT Proxy)
- 55443 (gRPC for builder)

**Allowed Ingress**:
- OpenShift ingress controllers (port 80, 443)
- Mirror component (port 80, 443)
- Monitoring component (port 9091)

**Allowed Egress**:
- PostgreSQL database (port 5432)
- Redis cache (port 6379)
- Clair security scanner (ports 80, 8089)
- External object storage (ports 80, 443)
- DNS resolution (port 53)

### 2. PostgreSQL Database
**File**: `bundle/manifests/quay-network-policies.yaml` - `quay-postgres-netpol`

**Ports**:
- 5432 (PostgreSQL)

**Allowed Ingress**:
- Quay app (port 5432)
- Mirror component (port 5432)

**Allowed Egress**:
- DNS resolution (port 53)

### 3. Clair Security Scanner
**File**: `bundle/manifests/quay-network-policies.yaml` - `quay-clair-netpol`

**Ports**:
- 80 (HTTP API)
- 8089 (Introspection API)

**Allowed Ingress**:
- Quay app (ports 80, 8089)

**Allowed Egress**:
- Clair PostgreSQL (port 5432)
- External vulnerability databases (ports 80, 443)
- DNS resolution (port 53)

### 4. Clair PostgreSQL Database
**File**: `bundle/manifests/quay-network-policies.yaml` - `quay-clair-postgres-netpol`

**Ports**:
- 5432 (PostgreSQL)

**Allowed Ingress**:
- Clair app (port 5432)

**Allowed Egress**:
- DNS resolution (port 53)

### 5. Redis Cache
**File**: `bundle/manifests/quay-network-policies.yaml` - `quay-redis-netpol`

**Ports**:
- 6379 (Redis)

**Allowed Ingress**:
- Quay app (port 6379)
- Mirror component (port 6379)

**Allowed Egress**:
- DNS resolution (port 53)

### 6. Mirror Component
**File**: `bundle/manifests/quay-network-policies.yaml` - `quay-mirror-netpol`

**Allowed Egress**:
- Quay app (ports 80, 443)
- PostgreSQL (port 5432)
- Redis (port 6379)
- External registries for mirroring (ports 80, 443)
- DNS resolution (port 53)

### 7. Monitoring Component
**File**: `bundle/manifests/quay-network-policies.yaml` - `quay-monitoring-netpol`

**Ports**:
- 9091 (Metrics)

**Allowed Ingress**:
- OpenShift monitoring (port 9091)

**Allowed Egress**:
- Quay app for metrics collection (port 9091)
- DNS resolution (port 53)

### 8. Quay Operator
**File**: `bundle/manifests/quay-operator-network-policies.yaml` - `quay-operator-netpol`

**Ports**:
- 8080 (Metrics)
- 9443 (Webhook)

**Allowed Ingress**:
- Kubernetes API server (ports 8080, 9443)

**Allowed Egress**:
- Kubernetes API server (port 443)
- All Quay components for management
- DNS resolution (port 53)

## Security Principles

1. **Least Privilege**: Each component only receives the minimum network access required for its function
2. **Zero Trust**: No implicit trust between components - all communication must be explicitly allowed
3. **Defense in Depth**: Multiple layers of network security controls
4. **Default Deny**: A default deny-all policy ensures no unintended communication

## Deployment Instructions

1. Apply the main application network policies:
   ```bash
   kubectl apply -f bundle/manifests/quay-network-policies.yaml
   ```

2. Apply the operator network policies:
   ```bash
   kubectl apply -f bundle/manifests/quay-operator-network-policies.yaml
   ```

3. Verify the policies are applied:
   ```bash
   kubectl get networkpolicies
   ```

## Customization

The NetworkPolicies can be customized based on specific requirements:

- **External Access**: Modify ingress rules to allow specific external IP ranges
- **Additional Components**: Add new NetworkPolicies for additional components
- **Port Changes**: Update port numbers if component configurations change
- **Namespace Isolation**: Adjust namespace selectors for multi-tenant deployments

## Troubleshooting

If network connectivity issues occur after applying these policies:

1. Check NetworkPolicy status:
   ```bash
   kubectl describe networkpolicy <policy-name>
   ```

2. Verify pod labels match NetworkPolicy selectors:
   ```bash
   kubectl get pods --show-labels
   ```

3. Test connectivity between specific pods:
   ```bash
   kubectl exec -it <pod-name> -- nc -zv <target-pod-ip> <port>
   ```

4. Temporarily disable a specific NetworkPolicy for testing:
   ```bash
   kubectl delete networkpolicy <policy-name>
   ```

## Notes

- These NetworkPolicies are designed for OpenShift/Kubernetes environments
- External object storage access is allowed for S3-compatible storage
- DNS resolution is allowed for all components to support service discovery
- The policies assume standard OpenShift monitoring and ingress namespaces
- Consider additional policies for specific security requirements or compliance needs
