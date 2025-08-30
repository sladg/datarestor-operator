package controller

import (
	"context"
	"time"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// cleanupResources cleans up resources created by the PVCBackup
func (r *PVCBackupReconciler) cleanupResources(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup) error {
	logger := LoggerFrom(ctx, "cleanup").
		WithValues("name", pvcBackup.Name)

	// Clean up VolumeSnapshots created by this PVCBackup (if available)
	if r.isVolumeSnapshotAvailable(ctx) {
		logger.Starting("cleanup snapshots")
		var snapshots snapshotv1.VolumeSnapshotList
		if err := r.List(ctx, &snapshots, client.MatchingLabels(map[string]string{
			"pvcbackup.cheap-man-ha-store.com/created-by": pvcBackup.Name,
		})); err != nil {
			logger.Failed("list snapshots", err)
		} else {
			for _, snapshot := range snapshots.Items {
				snapshotLogger := logger.WithValues("snapshot", snapshot.Name)
				snapshotLogger.Debug("Deleting snapshot")
				if err := r.Delete(ctx, &snapshot); err != nil {
					snapshotLogger.Failed("delete snapshot", err)
				} else {
					snapshotLogger.Debug("Snapshot deleted")
				}
			}
		}
		logger.Completed("cleanup snapshots")
	} else {
		logger.Debug("VolumeSnapshot CRD not available")
	}

	// Clean up backup pods created by this PVCBackup
	var pods corev1.PodList
	logger.Starting("cleanup pods")
	if err := r.List(ctx, &pods, client.MatchingLabels(map[string]string{
		"pvcbackup.cheap-man-ha-store.com/created-by": pvcBackup.Name,
	})); err != nil {
		logger.Failed("list pods", err)
	} else {
		for _, pod := range pods.Items {
			podLogger := logger.WithValues("pod", pod.Name)
			podLogger.Debug("Deleting pod")
			if err := r.Delete(ctx, &pod); err != nil {
				podLogger.Failed("delete pod", err)
			} else {
				podLogger.Debug("Pod deleted")
			}
		}
	}
	logger.Completed("cleanup pods")

	return nil
}

// Helper functions (implementations would go here)
// handleDeletion handles cleanup when PVCBackup is being deleted
func (r *PVCBackupReconciler) handleDeletion(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "deletion").
		WithValues("name", pvcBackup.Name)

	logger.Starting("deletion")

	// Clean up any resources created by this PVCBackup
	if err := r.cleanupResources(ctx, pvcBackup); err != nil {
		logger.Failed("cleanup resources", err)
		// Don't return error to avoid blocking deletion
	}

	// Remove finalizer
	logger.Debug("Removing finalizer")
	controllerutil.RemoveFinalizer(pvcBackup, "pvcbackup.storage.cheap-man-ha-store.com/finalizer")
	if err := r.Update(ctx, pvcBackup); err != nil {
		logger.Failed("remove finalizer", err)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Completed("deletion")
	return ctrl.Result{}, nil
}

// isVolumeSnapshotAvailable checks if VolumeSnapshot CRD is available
func (r *PVCBackupReconciler) isVolumeSnapshotAvailable(ctx context.Context) bool {
	config := ctrl.GetConfigOrDie()
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return false
	}

	gv := schema.GroupVersion{Group: "snapshot.storage.k8s.io", Version: "v1"}
	resourceList, err := discoveryClient.ServerResourcesForGroupVersion(gv.String())
	if err != nil {
		return false
	}

	for _, resource := range resourceList.APIResources {
		if resource.Kind == "VolumeSnapshot" {
			return true
		}
	}
	return false
}

// shouldPerformBackup checks if it's time to perform a backup based on schedule
func (r *PVCBackupReconciler) shouldPerformBackup(pvcBackup *storagev1alpha1.PVCBackup) bool {
	// If no cron schedule, only perform manual backups
	if pvcBackup.Spec.Schedule.Cron == "" {
		return false
	}

	// Parse the cron schedule
	schedule, err := r.cronParser.Parse(pvcBackup.Spec.Schedule.Cron)
	if err != nil {
		return false
	}

	now := time.Now()

	// If there's no last backup time, perform backup now
	if pvcBackup.Status.LastBackup == nil {
		return true
	}

	// Check if it's time for the next backup
	nextBackup := schedule.Next(pvcBackup.Status.LastBackup.Time)
	return now.After(nextBackup) || now.Equal(nextBackup)
}
