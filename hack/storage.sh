#!/bin/bash
# deploys noobaa via openshift data foundation operator to a cluster from redhat
# marketplace.
#
# REQUIREMENTS:
#  * a valid login session to an OCP cluster, with cluster admin privileges
#  * `oc`

export TAG=${TAG:-"4"}
VERSION=$(oc version | grep 'Client Version' | cut -f2 -d ":" | cut -f2 -d ".")

# prints pre-formatted info output.
function info {
	echo "INFO $(date '+%Y-%m-%dT%H:%M:%S') $*"
}

cat <<EOF | oc apply -f -
apiVersion: v1
kind: Namespace
metadata:
  labels:
    kubernetes.io/metadata.name: openshift-storage
  name: openshift-storage
---
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  annotations:
  name: odf-og
  namespace: openshift-storage
spec:
  targetNamespaces:
  - openshift-storage
---
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  labels:
    operators.coreos.com/odf-operator.openshift-storage: ""
  name: odf-operator
  namespace: openshift-storage
spec:
  channel: stable-${TAG}.${VERSION}
  installPlanApproval: Automatic
  name: odf-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
EOF

NAMESPACE='openshift-storage'

info 'waiting for CSV installation...'

for _ in {1..60}; do
	phase="$(oc -n "${NAMESPACE}" get csv -l operators.coreos.com/odf-operator.openshift-storage -o jsonpath='{.items[*].status.phase}')"
	if [ "$phase" = "Succeeded" ]; then
		info "operator installed"
		break
	fi
	sleep 10
done

info 'creating noobaa object storage'

cat <<EOF | oc apply -f -
apiVersion: noobaa.io/v1alpha1
kind: NooBaa
metadata:
  name: noobaa
  namespace: openshift-storage
spec:
 dbType: postgres
 dbResources:
   requests:
     cpu: '0.1'
     memory: 1Gi
 coreResources:
   requests:
     cpu: '0.1'
     memory: 1Gi
EOF

info 'waiting for object store installation'

for _ in {1..60}; do
	phase="$(oc get noobaas noobaa -n "${NAMESPACE}" -o jsonpath='{.status.phase}')"
	if [ "$phase" = "Ready" ]; then
		info 'object store ready'
		break
	fi
	sleep 10
done

for _ in {1..60}; do
	phase="$(oc get backingstore noobaa-default-backing-store -n "${NAMESPACE}" -o jsonpath='{.status.phase}')"
	if [ "$phase" = "Ready" ]; then
		info 'backing store ready'
		break
	fi
	sleep 10
done

info 'install finished'
