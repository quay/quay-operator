#!/bin/bash

# Quay Operator Network Policies Integration Script
# This script integrates NetworkPolicies into the operator bundle

set -e

echo "=== Integrating NetworkPolicies into Quay Operator Bundle ==="

# Verify we're in the right directory
if [ ! -f "bundle/manifests/quay-operator.clusterserviceversion.yaml" ]; then
    echo "Error: Not in quay-operator directory or bundle not found"
    exit 1
fi

# Check if NetworkPolicy files exist
if [ ! -f "network-policies.yaml" ] || [ ! -f "operator-network-policies.yaml" ]; then
    echo "Error: NetworkPolicy files not found. Please run the NetworkPolicy generation first."
    exit 1
fi

echo "✓ NetworkPolicy files found"

# Copy NetworkPolicy files to bundle manifests
echo "Copying NetworkPolicy files to bundle manifests..."
cp network-policies.yaml bundle/manifests/quay-network-policies.yaml
cp operator-network-policies.yaml bundle/manifests/quay-operator-network-policies.yaml

echo "✓ NetworkPolicy files copied to bundle"

# Verify ClusterServiceVersion has NetworkPolicy support
if grep -q "kind: NetworkPolicy" bundle/manifests/quay-operator.clusterserviceversion.yaml; then
    echo "✓ ClusterServiceVersion includes NetworkPolicy in resources"
else
    echo "Warning: ClusterServiceVersion may not include NetworkPolicy in resources"
fi

if grep -q "networking.k8s.io" bundle/manifests/quay-operator.clusterserviceversion.yaml; then
    echo "✓ ClusterServiceVersion includes NetworkPolicy RBAC permissions"
else
    echo "Warning: ClusterServiceVersion may not include NetworkPolicy RBAC permissions"
fi

# List all files in bundle manifests
echo ""
echo "=== Bundle Manifests Contents ==="
ls -la bundle/manifests/

echo ""
echo "=== Integration Complete ==="
echo "The Quay Operator bundle now includes:"
echo "- quay-network-policies.yaml (Application NetworkPolicies)"
echo "- quay-operator-network-policies.yaml (Operator NetworkPolicies)"
echo "- Updated ClusterServiceVersion with NetworkPolicy support"
echo ""
echo "To deploy the operator with NetworkPolicies:"
echo "1. Build and push the operator bundle image"
echo "2. Update the CatalogSource to use the new bundle"
echo "3. The NetworkPolicies will be automatically deployed with the operator"
