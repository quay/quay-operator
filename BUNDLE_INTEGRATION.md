# Quay Operator Bundle Integration with NetworkPolicies

This document describes how NetworkPolicies have been integrated into the Quay Operator bundle for automatic deployment.

## Overview

The NetworkPolicies have been fully integrated into the Quay Operator bundle, ensuring that when the operator is installed, the network security policies are automatically deployed and managed.

## Integration Components

### 1. Bundle Manifests

The following files have been added to `bundle/manifests/`:

- **`quay-network-policies.yaml`** - Application-level NetworkPolicies for Quay components
- **`quay-operator-network-policies.yaml`** - Operator-specific NetworkPolicies

### 2. ClusterServiceVersion Updates

The `quay-operator.clusterserviceversion.yaml` has been updated to include:

#### Resources Declaration
```yaml
resources:
  - kind: Deployment
  - kind: ReplicaSet
  - kind: Pod
  - kind: Secret
  - kind: Job
  - kind: ConfigMap
  - kind: ServiceAccount
  - kind: PersistentVolumeClaim
  - kind: Ingress
  - kind: Route
  - kind: Role
  - kind: Rolebinding
  - kind: HorizontalPodAutoscaler
  - kind: ServiceMonitor
  - kind: PrometheusRule
  - kind: NetworkPolicy  # ← Added
```

#### RBAC Permissions
```yaml
- apiGroups:
    - networking.k8s.io
  resources:
    - networkpolicies
  verbs:
    - "*"
```

## NetworkPolicy Components

### Application NetworkPolicies (`quay-network-policies.yaml`)

1. **quay-app-netpol** - Main Quay application
2. **quay-postgres-netpol** - PostgreSQL database
3. **quay-clair-netpol** - Clair security scanner
4. **quay-clair-postgres-netpol** - Clair's database
5. **quay-redis-netpol** - Redis cache
6. **quay-mirror-netpol** - Repository mirroring
7. **quay-monitoring-netpol** - Metrics collection
8. **quay-default-deny-all** - Default deny policy

### Operator NetworkPolicies (`quay-operator-network-policies.yaml`)

1. **quay-operator-netpol** - Main operator controller
2. **quay-operator-webhook-netpol** - Admission controller webhook

## Deployment Process

### Automatic Deployment

When the Quay Operator is installed via OLM (Operator Lifecycle Manager), the NetworkPolicies are automatically deployed as part of the operator installation process.

### Manual Verification

To verify the NetworkPolicies are deployed:

```bash
# Check if NetworkPolicies exist
kubectl get networkpolicies

# Check specific NetworkPolicies
kubectl get networkpolicy quay-app-netpol
kubectl get networkpolicy quay-operator-netpol

# Describe a NetworkPolicy for details
kubectl describe networkpolicy quay-app-netpol
```

## Bundle Building

### Prerequisites

- Operator SDK installed
- Access to a container registry
- Kubernetes cluster with OLM

### Build and Push Bundle

```bash
# Build the bundle
operator-sdk bundle create quay-operator-bundle:v3.6.1

# Push to registry
docker push <registry>/quay-operator-bundle:v3.6.1

# Update CatalogSource
kubectl apply -f bundle/quay-operator.catalogsource.yaml
```

### Verify Bundle Contents

```bash
# Extract and inspect bundle contents
operator-sdk bundle validate quay-operator-bundle:v3.6.1

# List bundle contents
tar -tf quay-operator-bundle:v3.6.1 | grep -E "(network|Network)"
```

## Configuration

### Customizing NetworkPolicies

The NetworkPolicies can be customized by:

1. **Modifying the YAML files** in `bundle/manifests/`
2. **Rebuilding the bundle** with updated policies
3. **Updating the operator** to use the new bundle

### Environment-Specific Adjustments

Common customizations:

- **External IP ranges** for ingress access
- **Namespace selectors** for multi-tenant deployments
- **Port configurations** for custom component setups
- **Additional components** for extended functionality

## Troubleshooting

### NetworkPolicies Not Deployed

1. Check operator installation:
   ```bash
   kubectl get csv quay-operator.v3.6.1
   ```

2. Verify bundle contents:
   ```bash
   kubectl get bundle quay-operator-bundle
   ```

3. Check operator logs:
   ```bash
   kubectl logs -l app.kubernetes.io/name=quay-operator
   ```

### Connectivity Issues

1. Verify NetworkPolicy status:
   ```bash
   kubectl describe networkpolicy <policy-name>
   ```

2. Check pod labels match selectors:
   ```bash
   kubectl get pods --show-labels
   ```

3. Test connectivity:
   ```bash
   kubectl exec -it <pod-name> -- nc -zv <target-ip> <port>
   ```

## Security Benefits

The integrated NetworkPolicies provide:

1. **Zero-Trust Networking** - No implicit trust between components
2. **Defense in Depth** - Multiple layers of network security
3. **Least Privilege Access** - Only necessary network communication allowed
4. **Automatic Deployment** - No manual configuration required
5. **Consistent Security** - Same policies across all deployments

## Maintenance

### Updating NetworkPolicies

1. Modify the YAML files in `bundle/manifests/`
2. Update the bundle version in `quay-operator.clusterserviceversion.yaml`
3. Rebuild and push the bundle
4. Update the operator subscription

### Monitoring

Monitor NetworkPolicy effectiveness:

```bash
# Check NetworkPolicy violations
kubectl get events --field-selector reason=NetworkPolicyDenied

# Monitor network traffic
kubectl logs -l quay-component=quay-app | grep -i network
```

## Support

For issues related to NetworkPolicy integration:

1. Check the operator logs
2. Verify Kubernetes version compatibility (NetworkPolicies require v1.7+)
3. Ensure CNI plugin supports NetworkPolicies (Calico, Weave Net, etc.)
4. Review the troubleshooting section above

The NetworkPolicies are now fully integrated into the Quay Operator bundle and will be automatically deployed with every operator installation.
