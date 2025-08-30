#!/bin/bash

echo "Installing OpenEBS Local Path Storage and VolumeSnapshot CRDs..."

# Install OpenEBS operator
helm install openebs --namespace openebs openebs/openebs --set engines.replicated.mayastor.enabled=false --create-namespace

# Install VolumeSnapshot CRDs (required for the backup controller)
echo "Installing VolumeSnapshot CRDs..."
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/master/deploy/kubernetes/snapshot-controller/rbac-snapshot-controller.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/release-6.3/deploy/kubernetes/snapshot-controller/setup-snapshot-controller.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/release-6.3/client/config/crd/snapshot.storage.k8s.io_volumesnapshotclasses.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/release-6.3/client/config/crd/snapshot.storage.k8s.io_volumesnapshotcontents.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-csi/external-snapshotter/release-6.3/client/config/crd/snapshot.storage.k8s.io_volumesnapshots.yaml

kubectl delete crd volumesnapshotclasses.snapshot.storage.k8s.io
kubectl delete crd volumesnapshotcontents.snapshot.storage.k8s.io
kubectl delete crd volumesnapshots.snapshot.storage.k8s.io

# Wait for OpenEBS pods to be ready
echo "Waiting for OpenEBS pods to be ready..."
kubectl wait --for=condition=ready pod -l app=openebs-localpv-provisioner -n openebs --timeout=300s

echo "OpenEBS Local Path Storage and VolumeSnapshot CRDs installed successfully!"

helm install csi-driver-nfs csi-driver-nfs/csi-driver-nfs --namespace kube-system --version 4.11.0