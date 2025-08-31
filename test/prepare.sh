#!/bin/bash

echo "Installing OpenEBS Local Path Storage..."

# Install OpenEBS operator
helm install openebs --namespace openebs openebs/openebs --set engines.replicated.mayastor.enabled=false --create-namespace

# Wait for OpenEBS pods to be ready
echo "Waiting for OpenEBS pods to be ready..."
kubectl wait --for=condition=ready pod -l app=openebs-localpv-provisioner -n openebs --timeout=300s

echo "OpenEBS Local Path Storage installed successfully!"

helm install csi-driver-nfs csi-driver-nfs/csi-driver-nfs --namespace kube-system --version 4.11.0
