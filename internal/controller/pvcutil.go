package controller

import (
	"context"
	"fmt"
	"time"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// findMatchingPVCs finds PVCs that match the BackupConfig selector
func (r *BackupConfigReconciler) findMatchingPVCs(ctx context.Context, pvcBackup *backupv1alpha1.BackupConfig) ([]corev1.PersistentVolumeClaim, error) {
	logger := LoggerFrom(ctx, "pvc").
		WithValues("name", pvcBackup.Name)

	logger.Starting("find matching PVCs")
	var pvcs []corev1.PersistentVolumeClaim

	// Handle namespace-specific selection
	namespaces := pvcBackup.Spec.PVCSelector.Namespaces
	if len(namespaces) == 0 {
		// If no namespaces specified, search all namespaces
		namespaces = []string{""}
		logger.Debug("No namespaces specified, searching all")
	} else {
		logger.WithValues("namespaces", namespaces).Debug("Searching specific namespaces")
	}

	for _, namespace := range namespaces {
		nsLogger := logger.WithValues("namespace", namespace)
		var namespacePVCs corev1.PersistentVolumeClaimList

		if namespace == "" {
			// Search all namespaces
			if err := r.List(ctx, &namespacePVCs); err != nil {
				nsLogger.Failed("list PVCs across all namespaces", err)
				return nil, err
			}
		} else {
			// Search specific namespace
			if err := r.List(ctx, &namespacePVCs, client.InNamespace(namespace)); err != nil {
				nsLogger.Failed("list PVCs in namespace", err)
				return nil, err
			}
		}

		// Filter by label selector
		if pvcBackup.Spec.PVCSelector.LabelSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(pvcBackup.Spec.PVCSelector.LabelSelector)
			if err != nil {
				nsLogger.Failed("parse label selector", err)
				return nil, err
			}

			nsLogger.Debug("Filtering by label selector")
			for _, pvc := range namespacePVCs.Items {
				if selector.Matches(labels.Set(pvc.Labels)) {
					pvcs = append(pvcs, pvc)
				}
			}
		} else {
			// If no label selector, include all PVCs from the namespace
			nsLogger.Debug("No label selector, including all PVCs")
			pvcs = append(pvcs, namespacePVCs.Items...)
		}
	}

	// Filter by specific names if provided
	if len(pvcBackup.Spec.PVCSelector.Names) > 0 {
		logger.WithValues("names", pvcBackup.Spec.PVCSelector.Names).Debug("Filtering by specific names")
		var filteredPVCs []corev1.PersistentVolumeClaim
		nameSet := make(map[string]bool)
		for _, name := range pvcBackup.Spec.PVCSelector.Names {
			nameSet[name] = true
		}

		for _, pvc := range pvcs {
			if nameSet[pvc.Name] {
				filteredPVCs = append(filteredPVCs, pvc)
			}
		}
		pvcs = filteredPVCs
	}

	logger.WithValues("count", len(pvcs)).Completed("find matching PVCs")
	return pvcs, nil
}

func (r *BackupConfigReconciler) isPVCHealthy(ctx context.Context, pvc corev1.PersistentVolumeClaim) bool {
	logger := LoggerFrom(ctx, "pvc").
		WithValues("pvc", pvc.Name)

	logger.Starting("check health")

	if pvc.Status.Phase != corev1.ClaimBound {
		logger.WithValues("phase", pvc.Status.Phase).Debug("PVC is not bound")
		return false
	}

	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingFields{"spec.volumes.persistentVolumeClaim.claimName": pvc.Name}); err != nil {
		logger.Failed("list pods using PVC", err)
		return false
	}

	if len(pods.Items) == 0 {
		logger.Debug("No pods using PVC")
		logger.Completed("check health")
		return true // PVC is healthy if no pods are using it
	}

	for _, pod := range pods.Items {
		podLogger := logger.WithValues("pod", pod.Name)
		if !r.isPodHealthy(&pod) {
			podLogger.Debug("Pod is not healthy")
			return false
		}
	}

	logger.Completed("check health")
	return true
}

// waitForSnapshotReady waits for the VolumeSnapshot to be ready
func (r *BackupConfigReconciler) waitForSnapshotReady(ctx context.Context, snapshot *snapshotv1.VolumeSnapshot) error {
	logger := LoggerFrom(ctx, "snapshot").
		WithValues("name", snapshot.Name, "namespace", snapshot.Namespace)

	logger.Starting("wait for ready")

	// Wait up to 5 minutes for snapshot to be ready
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			logger.Failed("wait for ready", fmt.Errorf("timeout"))
			return fmt.Errorf("timeout waiting for snapshot to be ready: %s", snapshot.Name)
		case <-ticker.C:
			// Get the latest snapshot status
			var currentSnapshot snapshotv1.VolumeSnapshot
			if err := r.Get(ctx, types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, &currentSnapshot); err != nil {
				logger.Failed("get snapshot status", err)
				continue
			}

			if currentSnapshot.Status != nil && currentSnapshot.Status.ReadyToUse != nil && *currentSnapshot.Status.ReadyToUse {
				logger.Completed("wait for ready")
				return nil
			}

			if currentSnapshot.Status != nil && currentSnapshot.Status.Error != nil {
				err := fmt.Errorf("snapshot creation failed: %s", *currentSnapshot.Status.Error.Message)
				logger.Failed("wait for ready", err)
				return err
			}

			logger.Debug("Snapshot not ready yet")
		}
	}
}

// createVolumeSnapshot creates a VolumeSnapshot for the PVC
func (r *BackupConfigReconciler) createVolumeSnapshot(ctx context.Context, pvcBackup *backupv1alpha1.BackupConfig, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	logger := LoggerFrom(ctx, "snapshot").
		WithPVC(pvc).
		WithValues("name", pvcBackup.Name)

	logger.Starting("create snapshot")

	// Generate snapshot name
	snapshotName := fmt.Sprintf("%s-%s-%s", pvc.Name, pvc.Namespace, time.Now().Format("20060102-150405"))
	logger.WithValues("snapshot", snapshotName).Debug("Generated snapshot name")

	// Create VolumeSnapshot
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				"backupconfig.autorestore-backup-operator.com/created-by": pvcBackup.Name,
				"backupconfig.autorestore-backup-operator.com/pvc-name":   pvc.Name,
				"backupconfig.autorestore-backup-operator.com/timestamp":  time.Now().Format("20060102-150405"),
				"backupconfig.autorestore-backup-operator.com/target":     "volumesnapshot",
			},
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvc.Name,
			},
		},
	}

	if err := r.Create(ctx, snapshot); err != nil {
		logger.Failed("create snapshot", err)
		return nil, fmt.Errorf("failed to create VolumeSnapshot: %w", err)
	}

	logger.Debug("Created VolumeSnapshot")

	// Wait for snapshot to be ready
	if err := r.waitForSnapshotReady(ctx, snapshot); err != nil {
		logger.Failed("wait for snapshot ready", err)
		return nil, fmt.Errorf("snapshot not ready: %w", err)
	}

	logger.Completed("create snapshot")
	return snapshot, nil
}

// findObjectsForPVC finds BackupConfig objects for a given PVC
func (r *BackupConfigReconciler) findObjectsForPVC(ctx context.Context, obj client.Object) []reconcile.Request {
	pvc := obj.(*corev1.PersistentVolumeClaim)

	var pvcBackups backupv1alpha1.BackupConfigList
	if err := r.List(ctx, &pvcBackups); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, pvcBackup := range pvcBackups.Items {
		// Check if PVC matches the selector
		matchedPVCs, err := r.findMatchingPVCs(ctx, &pvcBackup)
		if err != nil {
			continue
		}

		for _, matchedPVC := range matchedPVCs {
			if matchedPVC.Name == pvc.Name && matchedPVC.Namespace == pvc.Namespace {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      pvcBackup.Name,
						Namespace: pvcBackup.Namespace,
					},
				})
				break
			}
		}
	}

	return requests
}
