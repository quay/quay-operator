#!/bin/bash

# Quay Operator Network Policy Validation Script
# This script helps validate that NetworkPolicies are working correctly

set -e

NAMESPACE=${1:-quay-operator}
echo "Validating NetworkPolicies in namespace: $NAMESPACE"

# Function to check if a resource exists
check_resource() {
    local resource_type=$1
    local resource_name=$2
    if kubectl get $resource_type $resource_name -n $NAMESPACE >/dev/null 2>&1; then
        echo "✓ $resource_type/$resource_name exists"
        return 0
    else
        echo "✗ $resource_type/$resource_name not found"
        return 1
    fi
}

# Function to test network connectivity
test_connectivity() {
    local source_pod=$1
    local target_pod=$2
    local port=$3
    local protocol=${4:-TCP}
    
    echo "Testing connectivity from $source_pod to $target_pod:$port ($protocol)"
    
    if kubectl exec -n $NAMESPACE $source_pod -- nc -z -v $target_pod $port 2>&1 | grep -q "succeeded"; then
        echo "✓ Connection successful"
        return 0
    else
        echo "✗ Connection failed"
        return 1
    fi
}

echo "=== Checking NetworkPolicy Resources ==="

# Check if NetworkPolicies exist
check_resource "networkpolicy" "quay-app-netpol"
check_resource "networkpolicy" "quay-postgres-netpol"
check_resource "networkpolicy" "quay-clair-netpol"
check_resource "networkpolicy" "quay-clair-postgres-netpol"
check_resource "networkpolicy" "quay-redis-netpol"
check_resource "networkpolicy" "quay-mirror-netpol"
check_resource "networkpolicy" "quay-monitoring-netpol"
check_resource "networkpolicy" "quay-default-deny-all"

echo ""
echo "=== Checking Pod Labels ==="

# Check if pods have correct labels
echo "Checking Quay app pods:"
kubectl get pods -n $NAMESPACE -l quay-component=quay-app --show-labels

echo ""
echo "Checking PostgreSQL pods:"
kubectl get pods -n $NAMESPACE -l quay-component=postgres --show-labels

echo ""
echo "Checking Clair pods:"
kubectl get pods -n $NAMESPACE -l quay-component=clair-app --show-labels

echo ""
echo "Checking Redis pods:"
kubectl get pods -n $NAMESPACE -l quay-component=redis --show-labels

echo ""
echo "=== Network Policy Details ==="

# Show NetworkPolicy details
for policy in quay-app-netpol quay-postgres-netpol quay-clair-netpol quay-redis-netpol; do
    echo "--- $policy ---"
    kubectl get networkpolicy $policy -n $NAMESPACE -o yaml | grep -A 20 "spec:"
    echo ""
done

echo "=== Connectivity Tests ==="
echo "Note: These tests require pods to be running and may need to be run manually"
echo "Example commands:"
echo "kubectl exec -n $NAMESPACE <quay-pod> -- nc -z -v <postgres-pod-ip> 5432"
echo "kubectl exec -n $NAMESPACE <quay-pod> -- nc -z -v <redis-pod-ip> 6379"
echo "kubectl exec -n $NAMESPACE <quay-pod> -- nc -z -v <clair-pod-ip> 80"

echo ""
echo "=== Validation Complete ==="
echo "If all resources exist and pods have correct labels, the NetworkPolicies should be working correctly."
echo "For detailed connectivity testing, run the example commands above with actual pod names and IPs."
