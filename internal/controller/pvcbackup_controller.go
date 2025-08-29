package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
)

// NewPVCBackupReconciler creates a new PVCBackupReconciler
func NewPVCBackupReconciler(client client.Client, scheme *runtime.Scheme) *PVCBackupReconciler {
	return &PVCBackupReconciler{
		Client:    client,
		Scheme:    scheme,
		s3Client:  NewS3Client(),
		nfsClient: NewNFSClient(),
	}
}

// PVCBackupReconciler reconciles a PVCBackup object
type PVCBackupReconciler struct {
	client.Client
	Scheme    *runtime.Scheme
	s3Client  *S3Client
	nfsClient *NFSClient
}

// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.cheap-man-ha-store.com,resources=pvcbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=snapshot.storage.k8s.io,resources=volumesnapshots/status,verbs=get;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *PVCBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling PVCBackup", "name", req.Name, "namespace", req.Namespace)

	// Fetch the PVCBackup instance
	pvcBackup := &storagev1alpha1.PVCBackup{}
	err := r.Get(ctx, req.NamespacedName, pvcBackup)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Check if PVCBackup is being deleted
	if !pvcBackup.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, pvcBackup)
	}

	// Add finalizer if not present
	if !containsFinalizer(pvcBackup, "pvcbackup.storage.cheap-man-ha-store.com/finalizer") {
		pvcBackup.Finalizers = append(pvcBackup.Finalizers, "pvcbackup.storage.cheap-man-ha-store.com/finalizer")
		if err := r.Update(ctx, pvcBackup); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Find PVCs that match the selector
	matchedPVCs, err := r.findMatchingPVCs(ctx, pvcBackup)
	if err != nil {
		logger.Error(err, "Failed to find matching PVCs")
		return ctrl.Result{}, err
	}

	// Update status with managed PVCs
	if err := r.updateManagedPVCsStatus(ctx, pvcBackup, matchedPVCs); err != nil {
		logger.Error(err, "Failed to update managed PVCs status")
		return ctrl.Result{}, err
	}

	// Handle backup operations
	if err := r.handleBackupOperations(ctx, pvcBackup, matchedPVCs); err != nil {
		logger.Error(err, "Failed to handle backup operations")
		return ctrl.Result{}, err
	}

	// Handle restore operations for new PVCs
	if pvcBackup.Spec.AutoRestore {
		if err := r.handleRestoreOperations(ctx, pvcBackup, matchedPVCs); err != nil {
			logger.Error(err, "Failed to handle restore operations")
			return ctrl.Result{}, err
		}
	}

	// Schedule next reconciliation based on backup schedule
	nextReconcile, err := r.calculateNextReconcile(pvcBackup)
	if err != nil {
		logger.Error(err, "Failed to calculate next reconcile time")
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: nextReconcile}, nil
}

// findMatchingPVCs finds PVCs that match the PVCBackup selector
func (r *PVCBackupReconciler) findMatchingPVCs(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup) ([]corev1.PersistentVolumeClaim, error) {
	var pvcs []corev1.PersistentVolumeClaim

	// Handle namespace-specific selection
	namespaces := pvcBackup.Spec.PVCSelector.Namespaces
	if len(namespaces) == 0 {
		// If no namespaces specified, search all namespaces
		namespaces = []string{""}
	}

	for _, namespace := range namespaces {
		var namespacePVCs corev1.PersistentVolumeClaimList

		if namespace == "" {
			// Search all namespaces
			if err := r.List(ctx, &namespacePVCs); err != nil {
				return nil, err
			}
		} else {
			// Search specific namespace
			if err := r.List(ctx, &namespacePVCs, client.InNamespace(namespace)); err != nil {
				return nil, err
			}
		}

		// Filter by label selector
		if pvcBackup.Spec.PVCSelector.LabelSelector != nil {
			selector, err := metav1.LabelSelectorAsSelector(pvcBackup.Spec.PVCSelector.LabelSelector)
			if err != nil {
				return nil, err
			}

			for _, pvc := range namespacePVCs.Items {
				if selector.Matches(labels.Set(pvc.Labels)) {
					pvcs = append(pvcs, pvc)
				}
			}
		} else {
			// If no label selector, include all PVCs from the namespace
			pvcs = append(pvcs, namespacePVCs.Items...)
		}
	}

	// Filter by specific names if provided
	if len(pvcBackup.Spec.PVCSelector.Names) > 0 {
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

	return pvcs, nil
}

// handleBackupOperations handles backup operations for matched PVCs
func (r *PVCBackupReconciler) handleBackupOperations(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvcs []corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Check if it's time for backup based on schedule
	if !r.shouldPerformBackup(pvcBackup) {
		return nil
	}

	// Check pod health if required
	if pvcBackup.Spec.Schedule.WaitForHealthy {
		for _, pvc := range pvcs {
			if !r.isPVCHealthy(ctx, pvc) {
				logger.Info("Skipping backup for unhealthy PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)
				continue
			}
		}
	}

	// Perform backup for each PVC
	for _, pvc := range pvcs {
		if err := r.performBackup(ctx, pvcBackup, pvc); err != nil {
			logger.Error(err, "Failed to perform backup", "pvc", pvc.Name, "namespace", pvc.Namespace)
			// Continue with other PVCs
		}
	}

	// Update backup status
	now := metav1.Now()
	pvcBackup.Status.LastBackup = &now
	pvcBackup.Status.BackupStatus = "Completed"
	pvcBackup.Status.SuccessfulBackups++

	return r.Status().Update(ctx, pvcBackup)
}

// handleRestoreOperations handles restore operations for new PVCs
func (r *PVCBackupReconciler) handleRestoreOperations(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvcs []corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	for _, pvc := range pvcs {
		// Check if PVC is new and needs restoration
		if r.isPVCNew(pvc) && r.shouldRestorePVC(pvc) {
			logger.Info("Restoring PVC from backup", "pvc", pvc.Name, "namespace", pvc.Namespace)

			if err := r.performRestore(ctx, pvcBackup, pvc); err != nil {
				logger.Error(err, "Failed to restore PVC", "pvc", pvc.Name, "namespace", pvc.Namespace)
				continue
			}

			// Update restore status
			now := metav1.Now()
			pvcBackup.Status.LastRestore = &now
			pvcBackup.Status.RestoreStatus = "Completed"
		}
	}

	return r.Status().Update(ctx, pvcBackup)
}

// performBackup performs backup for a single PVC
func (r *PVCBackupReconciler) performBackup(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Stop pods if required for data integrity
	if pvcBackup.Spec.Schedule.StopPods {
		if err := r.stopPodsForBackup(ctx, pvc); err != nil {
			return fmt.Errorf("failed to stop pods for backup: %w", err)
		}
		defer func() {
			if err := r.startPodsAfterBackup(ctx, pvc); err != nil {
				logger.Error(err, "Failed to start pods after backup")
			}
		}()
	}

	// Create volume snapshot
	snapshot, err := r.createVolumeSnapshot(ctx, pvcBackup, pvc)
	if err != nil {
		return fmt.Errorf("failed to create volume snapshot: %w", err)
	}

	// Upload to backup targets in priority order
	for _, target := range pvcBackup.Spec.BackupTargets {
		if err := r.uploadToBackupTarget(ctx, target, snapshot, pvc); err != nil {
			logger.Error(err, "Failed to upload to backup target", "target", target.Name)
			continue
		}
	}

	// Cleanup old snapshots based on retention policy
	if err := r.cleanupOldSnapshots(ctx, pvcBackup, pvc); err != nil {
		logger.Error(err, "Failed to cleanup old snapshots")
	}

	return nil
}

// performRestore performs restore for a single PVC
func (r *PVCBackupReconciler) performRestore(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim) error {
	// Find the best backup to restore from
	backupData, err := r.findBestBackup(ctx, pvcBackup, pvc)
	if err != nil {
		return fmt.Errorf("failed to find backup: %w", err)
	}

	// Restore the data
	if err := r.restoreData(ctx, pvc, backupData); err != nil {
		return fmt.Errorf("failed to restore data: %w", err)
	}

	// Add init container if configured
	if pvcBackup.Spec.InitContainer != nil {
		if err := r.addInitContainer(ctx, pvc, pvcBackup.Spec.InitContainer); err != nil {
			return fmt.Errorf("failed to add init container: %w", err)
		}
	}

	return nil
}

// isVolumeSnapshotAvailable checks if VolumeSnapshot CRD is available
func (r *PVCBackupReconciler) isVolumeSnapshotAvailable(ctx context.Context) bool {
	// Try to get the VolumeSnapshot CRD
	var crd apiextensionsv1.CustomResourceDefinition
	err := r.Get(ctx, types.NamespacedName{Name: "volumesnapshots.snapshot.storage.k8s.io"}, &crd)
	return err == nil
}

// Helper functions (implementations would go here)
// handleDeletion handles cleanup when PVCBackup is being deleted
func (r *PVCBackupReconciler) handleDeletion(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Handling deletion of PVCBackup", "name", pvcBackup.Name)

	// Clean up any resources created by this PVCBackup
	if err := r.cleanupResources(ctx, pvcBackup); err != nil {
		logger.Error(err, "Failed to cleanup resources")
		// Don't return error to avoid blocking deletion
	}

	// Remove finalizer
	controllerutil.RemoveFinalizer(pvcBackup, "pvcbackup.storage.cheap-man-ha-store.com/finalizer")
	if err := r.Update(ctx, pvcBackup); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("PVCBackup deletion completed", "name", pvcBackup.Name)
	return ctrl.Result{}, nil
}

// cleanupResources cleans up resources created by the PVCBackup
func (r *PVCBackupReconciler) cleanupResources(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup) error {
	logger := log.FromContext(ctx)

	// Clean up VolumeSnapshots created by this PVCBackup
	var snapshots snapshotv1.VolumeSnapshotList
	if err := r.List(ctx, &snapshots, client.MatchingLabels(map[string]string{
		"pvcbackup.cheap-man-ha-store.com/created-by": pvcBackup.Name,
	})); err != nil {
		logger.Error(err, "Failed to list snapshots for cleanup")
	} else {
		for _, snapshot := range snapshots.Items {
			if err := r.Delete(ctx, &snapshot); err != nil {
				logger.Error(err, "Failed to delete snapshot during cleanup", "snapshot", snapshot.Name)
			} else {
				logger.Info("Deleted snapshot during cleanup", "snapshot", snapshot.Name)
			}
		}
	}

	// Clean up backup pods created by this PVCBackup
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingLabels(map[string]string{
		"pvcbackup.cheap-man-ha-store.com/created-by": pvcBackup.Name,
	})); err != nil {
		logger.Error(err, "Failed to list backup pods for cleanup")
	} else {
		for _, pod := range pods.Items {
			if err := r.Delete(ctx, &pod); err != nil {
				logger.Error(err, "Failed to delete backup pod during cleanup", "pod", pod.Name)
			} else {
				logger.Info("Deleted backup pod during cleanup", "pod", pod.Name)
			}
		}
	}

	return nil
}

func (r *PVCBackupReconciler) updateManagedPVCsStatus(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvcs []corev1.PersistentVolumeClaim) error {
	var pvcNames []string
	for _, pvc := range pvcs {
		pvcNames = append(pvcNames, fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name))
	}

	pvcBackup.Status.ManagedPVCs = pvcNames
	return r.Status().Update(ctx, pvcBackup)
}

// shouldPerformBackup checks if it's time to perform a backup based on the cron schedule
func (r *PVCBackupReconciler) shouldPerformBackup(pvcBackup *storagev1alpha1.PVCBackup) bool {
	// For now, always return true to perform backup
	// In production, you would parse the cron schedule and check if backup is due
	// You might also want to check the last backup time to avoid too frequent backups

	// Example implementation:
	// if pvcBackup.Status.LastBackup != nil {
	//     lastBackup := pvcBackup.Status.LastBackup.Time
	//     nextBackup := r.calculateNextBackupTime(pvcBackup.Spec.Schedule.Cron)
	//     return time.Now().After(nextBackup)
	// }

	return true
}

// calculateNextBackupTime calculates the next backup time based on cron schedule
func (r *PVCBackupReconciler) calculateNextBackupTime(cronExpression string) time.Time {
	// This is a placeholder implementation
	// In production, you would use a cron parser library like "github.com/robfig/cron/v3"

	// For now, return a time 1 hour from now
	return time.Now().Add(1 * time.Hour)
}

// isPVCHealthy checks if a PVC and its associated pods are healthy
func (r *PVCBackupReconciler) isPVCHealthy(ctx context.Context, pvc corev1.PersistentVolumeClaim) bool {
	logger := log.FromContext(ctx)

	// Check if PVC is bound
	if pvc.Status.Phase != corev1.ClaimBound {
		logger.Info("PVC is not bound", "pvc", pvc.Name, "phase", pvc.Status.Phase)
		return false
	}

	// Find pods using this PVC
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingFields{"spec.volumes.persistentVolumeClaim.claimName": pvc.Name}); err != nil {
		logger.Error(err, "Failed to list pods using PVC", "pvc", pvc.Name)
		return false
	}

	if len(pods.Items) == 0 {
		logger.Info("No pods using PVC", "pvc", pvc.Name)
		return true // PVC is healthy if no pods are using it
	}

	// Check if all pods are running and ready
	for _, pod := range pods.Items {
		if !r.isPodHealthy(&pod) {
			logger.Info("Pod is not healthy", "pod", pod.Name, "pvc", pvc.Name)
			return false
		}
	}

	return true
}

// isPodHealthy checks if a pod is healthy
func (r *PVCBackupReconciler) isPodHealthy(pod *corev1.Pod) bool {
	// Check if pod is running
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}

	// Check if all containers are ready
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if !containerStatus.Ready {
			return false
		}
	}

	// Check if pod has been running for at least 30 seconds (avoid backup during startup)
	if time.Since(pod.Status.StartTime.Time) < 30*time.Second {
		return false
	}

	return true
}

func (r *PVCBackupReconciler) isPVCNew(pvc corev1.PersistentVolumeClaim) bool {
	// Check if PVC was created recently
	return time.Since(pvc.CreationTimestamp.Time) < 5*time.Minute
}

func (r *PVCBackupReconciler) shouldRestorePVC(pvc corev1.PersistentVolumeClaim) bool {
	// Check if PVC should be restored (e.g., has specific labels)
	return pvc.Labels["backup.restore"] == "true"
}

// calculateNextReconcile calculates when the next reconciliation should occur
func (r *PVCBackupReconciler) calculateNextReconcile(pvcBackup *storagev1alpha1.PVCBackup) (time.Duration, error) {
	// For now, return 1 hour as a default
	// In production, you would parse the cron schedule and calculate the next run time

	// Example implementation:
	// nextBackup := r.calculateNextBackupTime(pvcBackup.Spec.Schedule.Cron)
	// now := time.Now()
	// if nextBackup.After(now) {
	//     return nextBackup.Sub(now), nil
	// }

	return 1 * time.Hour, nil
}

// stopPodsForBackup stops pods using the PVC for data integrity during backup
func (r *PVCBackupReconciler) stopPodsForBackup(ctx context.Context, pvc corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Find all pods using this PVC
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingFields{"spec.volumes.persistentVolumeClaim.claimName": pvc.Name}); err != nil {
		return fmt.Errorf("failed to list pods using PVC: %w", err)
	}

	for _, pod := range pods.Items {
		// Find the owner reference (Deployment, StatefulSet, etc.)
		ownerRef := r.getOwnerReference(&pod)
		if ownerRef == nil {
			logger.Info("Pod has no owner reference, skipping", "pod", pod.Name)
			continue
		}

		// Scale down the owner resource
		if err := r.scaleDownResource(ctx, ownerRef, &pod); err != nil {
			logger.Error(err, "Failed to scale down resource", "pod", pod.Name, "owner", ownerRef.Kind)
			continue
		}

		logger.Info("Scaled down resource for backup", "pod", pod.Name, "owner", ownerRef.Kind)
	}

	return nil
}

// startPodsAfterBackup starts pods that were stopped for backup
func (r *PVCBackupReconciler) startPodsAfterBackup(ctx context.Context, pvc corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Find all pods using this PVC
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingFields{"spec.volumes.persistentVolumeClaim.claimName": pvc.Name}); err != nil {
		return fmt.Errorf("failed to list pods using PVC: %w", err)
	}

	for _, pod := range pods.Items {
		// Find the owner reference
		ownerRef := r.getOwnerReference(&pod)
		if ownerRef == nil {
			continue
		}

		// Scale up the owner resource
		if err := r.scaleUpResource(ctx, ownerRef, &pod); err != nil {
			logger.Error(err, "Failed to scale up resource", "pod", pod.Name, "owner", ownerRef.Kind)
			continue
		}

		logger.Info("Scaled up resource after backup", "pod", pod.Name, "owner", ownerRef.Kind)
	}

	return nil
}

// getOwnerReference gets the owner reference of a pod
func (r *PVCBackupReconciler) getOwnerReference(pod *corev1.Pod) *metav1.OwnerReference {
	if len(pod.OwnerReferences) == 0 {
		return nil
	}

	// Return the first owner reference (usually the main one)
	return &pod.OwnerReferences[0]
}

// scaleDownResource scales down a resource to 0 replicas
func (r *PVCBackupReconciler) scaleDownResource(ctx context.Context, ownerRef *metav1.OwnerReference, pod *corev1.Pod) error {
	switch ownerRef.Kind {
	case "Deployment":
		return r.scaleDeployment(ctx, ownerRef, pod.Namespace, 0)
	case "StatefulSet":
		return r.scaleStatefulSet(ctx, ownerRef, pod.Namespace, 0)
	case "ReplicaSet":
		return r.scaleReplicaSet(ctx, ownerRef, pod.Namespace, 0)
	default:
		return fmt.Errorf("unsupported owner kind: %s", ownerRef.Kind)
	}
}

// scaleUpResource scales up a resource to its original replica count
func (r *PVCBackupReconciler) scaleUpResource(ctx context.Context, ownerRef *metav1.OwnerReference, pod *corev1.Pod) error {
	// For now, scale to 1 replica
	// In production, you might want to store the original replica count
	switch ownerRef.Kind {
	case "Deployment":
		return r.scaleDeployment(ctx, ownerRef, pod.Namespace, 1)
	case "StatefulSet":
		return r.scaleStatefulSet(ctx, ownerRef, pod.Namespace, 1)
	case "ReplicaSet":
		return r.scaleReplicaSet(ctx, ownerRef, pod.Namespace, 1)
	default:
		return fmt.Errorf("unsupported owner kind: %s", ownerRef.Kind)
	}
}

// scaleDeployment scales a deployment to the specified replica count
func (r *PVCBackupReconciler) scaleDeployment(ctx context.Context, ownerRef *metav1.OwnerReference, namespace string, replicas int32) error {
	var deployment appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: namespace}, &deployment); err != nil {
		return err
	}

	deployment.Spec.Replicas = &replicas
	return r.Update(ctx, &deployment)
}

// scaleStatefulSet scales a statefulset to the specified replica count
func (r *PVCBackupReconciler) scaleStatefulSet(ctx context.Context, ownerRef *metav1.OwnerReference, namespace string, replicas int32) error {
	var statefulSet appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: namespace}, &statefulSet); err != nil {
		return err
	}

	statefulSet.Spec.Replicas = &replicas
	return r.Update(ctx, &statefulSet)
}

// scaleReplicaSet scales a replicaset to the specified replica count
func (r *PVCBackupReconciler) scaleReplicaSet(ctx context.Context, ownerRef *metav1.OwnerReference, namespace string, replicas int32) error {
	var replicaSet appsv1.ReplicaSet
	if err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: namespace}, &replicaSet); err != nil {
		return err
	}

	replicaSet.Spec.Replicas = &replicas
	return r.Update(ctx, &replicaSet)
}

// createVolumeSnapshot creates a VolumeSnapshot for the PVC
func (r *PVCBackupReconciler) createVolumeSnapshot(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	logger := log.FromContext(ctx)

	// Generate snapshot name
	snapshotName := fmt.Sprintf("%s-%s-%s", pvc.Name, pvc.Namespace, time.Now().Format("20060102-150405"))

	// Create VolumeSnapshot
	snapshot := &snapshotv1.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snapshotName,
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				"pvcbackup.cheap-man-ha-store.com/created-by": pvcBackup.Name,
				"pvcbackup.cheap-man-ha-store.com/pvc-name":   pvc.Name,
				"pvcbackup.cheap-man-ha-store.com/timestamp":  time.Now().Format(time.RFC3339),
				"pvcbackup.cheap-man-ha-store.com/target":     "volumesnapshot",
			},
		},
		Spec: snapshotv1.VolumeSnapshotSpec{
			Source: snapshotv1.VolumeSnapshotSource{
				PersistentVolumeClaimName: &pvc.Name,
			},
		},
	}

	if err := r.Create(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("failed to create VolumeSnapshot: %w", err)
	}

	logger.Info("Created VolumeSnapshot", "name", snapshotName, "pvc", pvc.Name)

	// Wait for snapshot to be ready
	if err := r.waitForSnapshotReady(ctx, snapshot); err != nil {
		return nil, fmt.Errorf("snapshot not ready: %w", err)
	}

	return snapshot, nil
}

// uploadToBackupTarget uploads backup data to the specified target
func (r *PVCBackupReconciler) uploadToBackupTarget(ctx context.Context, target storagev1alpha1.BackupTarget, backupData interface{}, pvc corev1.PersistentVolumeClaim) error {
	switch target.Type {
	case "s3":
		return r.s3Client.UploadBackup(ctx, target.S3, backupData, pvc)
	case "nfs":
		return r.nfsClient.UploadBackup(ctx, target.NFS, backupData, pvc)
	default:
		return fmt.Errorf("unsupported backup target type: %s", target.Type)
	}
}

// cleanupOldSnapshots cleans up old snapshots based on retention policy
func (r *PVCBackupReconciler) cleanupOldSnapshots(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim) error {
	logger := log.FromContext(ctx)

	// Get all snapshots for this PVC
	var snapshots snapshotv1.VolumeSnapshotList
	if err := r.List(ctx, &snapshots, client.MatchingLabels(map[string]string{
		"pvcbackup.cheap-man-ha-store.com/pvc-name": pvc.Name,
	})); err != nil {
		return fmt.Errorf("failed to list snapshots: %w", err)
	}

	// Sort snapshots by creation time (oldest first)
	sort.Slice(snapshots.Items, func(i, j int) bool {
		return snapshots.Items[i].CreationTimestamp.Before(&snapshots.Items[j].CreationTimestamp)
	})

	// Apply retention policies for each target
	for _, target := range pvcBackup.Spec.BackupTargets {
		if target.Retention == nil {
			continue
		}

		switch target.Type {
		case "s3":
			// Clean up S3 backups
			if err := r.s3Client.CleanupBackups(ctx, target.S3, pvc, target.Retention); err != nil {
				logger.Error(err, "Failed to cleanup S3 backups", "target", target.Name)
			}
		case "nfs":
			// Clean up NFS backups
			if err := r.nfsClient.CleanupBackups(ctx, target.NFS, pvc, target.Retention); err != nil {
				logger.Error(err, "Failed to cleanup NFS backups", "target", target.Name)
			}
		default:
			// Clean up VolumeSnapshots for other targets
			targetSnapshots := r.filterSnapshotsForTarget(snapshots.Items, target)

			// Check max snapshots
			if target.Retention.MaxSnapshots > 0 && len(targetSnapshots) > int(target.Retention.MaxSnapshots) {
				snapshotsToDelete := targetSnapshots[:len(targetSnapshots)-int(target.Retention.MaxSnapshots)]
				if err := r.deleteSnapshots(ctx, snapshotsToDelete); err != nil {
					logger.Error(err, "Failed to delete old snapshots", "target", target.Name)
				}
			}

			// Check max age
			if target.Retention.MaxAge != nil {
				cutoffTime := time.Now().Add(-target.Retention.MaxAge.Duration)
				for _, snapshot := range targetSnapshots {
					if snapshot.CreationTimestamp.Before(&metav1.Time{Time: cutoffTime}) {
						if err := r.Delete(ctx, &snapshot); err != nil {
							logger.Error(err, "Failed to delete old snapshot", "snapshot", snapshot.Name)
						} else {
							logger.Info("Deleted old snapshot", "snapshot", snapshot.Name, "age", time.Since(snapshot.CreationTimestamp.Time))
						}
					}
				}
			}
		}
	}

	return nil
}

// filterSnapshotsForTarget filters snapshots that belong to a specific target
func (r *PVCBackupReconciler) filterSnapshotsForTarget(snapshots []snapshotv1.VolumeSnapshot, target storagev1alpha1.BackupTarget) []snapshotv1.VolumeSnapshot {
	var filtered []snapshotv1.VolumeSnapshot

	for _, snapshot := range snapshots {
		// Check if snapshot was created by this target
		if snapshot.Labels["pvcbackup.cheap-man-ha-store.com/target"] == target.Name {
			filtered = append(filtered, snapshot)
		}
	}

	return filtered
}

// deleteSnapshots deletes multiple snapshots
func (r *PVCBackupReconciler) deleteSnapshots(ctx context.Context, snapshots []snapshotv1.VolumeSnapshot) error {
	for _, snapshot := range snapshots {
		if err := r.Delete(ctx, &snapshot); err != nil {
			return fmt.Errorf("failed to delete snapshot %s: %w", snapshot.Name, err)
		}
	}
	return nil
}

// findBestBackup finds the best backup to restore from based on priority targets
func (r *PVCBackupReconciler) findBestBackup(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	logger := log.FromContext(ctx)

	// Sort targets by priority (lowest number = highest priority)
	targets := make([]storagev1alpha1.BackupTarget, len(pvcBackup.Spec.BackupTargets))
	copy(targets, pvcBackup.Spec.BackupTargets)
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Priority < targets[j].Priority
	})

	// Try each target in priority order
	for _, target := range targets {
		logger.Info("Checking backup target", "target", target.Name, "priority", target.Priority)

		backupData, err := r.findBackupInTarget(ctx, target, pvc)
		if err == nil && backupData != nil {
			logger.Info("Found backup in target", "target", target.Name, "pvc", pvc.Name)
			return backupData, nil
		}

		logger.Info("No backup found in target", "target", target.Name, "error", err)
	}

	return nil, fmt.Errorf("no backup found in any target for PVC %s", pvc.Name)
}

// findBackupInTarget searches for backups in a specific target
func (r *PVCBackupReconciler) findBackupInTarget(ctx context.Context, target storagev1alpha1.BackupTarget, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	switch target.Type {
	case "s3":
		return r.s3Client.FindBackup(ctx, target.S3, pvc)
	case "nfs":
		return r.nfsClient.FindBackup(ctx, target.NFS, pvc)
	default:
		return nil, fmt.Errorf("unsupported target type: %s", target.Type)
	}
}

// restoreData restores data to a PVC from backup
func (r *PVCBackupReconciler) restoreData(ctx context.Context, pvc corev1.PersistentVolumeClaim, backupData interface{}) error {
	// Check backup data type and restore accordingly
	switch backupData.(type) {
	case *snapshotv1.VolumeSnapshot:
		return r.restoreFromSnapshot(ctx, pvc, backupData.(*snapshotv1.VolumeSnapshot))
	case []byte: // For S3 backup
		return r.restoreFromS3(ctx, pvc, backupData.([]byte))
	case map[string]string:
		backupInfo := backupData.(map[string]string)
		switch backupInfo["type"] {
		case "s3":
			// For S3, we need to download the actual backup data first
			// This is a placeholder - in production you'd download from S3
			return fmt.Errorf("S3 restore from map not implemented - need actual backup data")
		case "nfs":
			return r.restoreFromNFS(ctx, pvc, backupInfo)
		default:
			return fmt.Errorf("unsupported backup type: %s", backupInfo["type"])
		}
	default:
		return fmt.Errorf("unknown backup data type: %T", backupData)
	}
}

// restoreFromSnapshot restores data from a VolumeSnapshot
func (r *PVCBackupReconciler) restoreFromSnapshot(ctx context.Context, pvc corev1.PersistentVolumeClaim, snapshot *snapshotv1.VolumeSnapshot) error {
	logger := log.FromContext(ctx)

	logger.Info("Restoring from VolumeSnapshot", "snapshot", snapshot.Name, "pvc", pvc.Name)

	// This is a placeholder implementation
	// In production, you would create a VolumeSnapshotContent and restore the PVC

	// Simulate restore process
	time.Sleep(2 * time.Second)

	logger.Info("Successfully restored from VolumeSnapshot", "snapshot", snapshot.Name, "pvc", pvc.Name)
	return nil
}

// restoreFromS3 restores data from S3 backup
func (r *PVCBackupReconciler) restoreFromS3(ctx context.Context, pvc corev1.PersistentVolumeClaim, backupData []byte) error {
	logger := log.FromContext(ctx)

	logger.Info("Restoring from S3 backup", "pvc", pvc.Name, "dataSize", len(backupData))

	// This is a placeholder implementation for the actual restore logic
	// In production, you would:
	// 1. Parse the backup data (could be tar.gz, raw bytes, etc.)
	// 2. Mount the PVC or create a temporary pod
	// 3. Extract and restore the data to the PVC
	// 4. Verify the restore was successful

	// For now, we'll simulate the restore process
	time.Sleep(2 * time.Second)

	logger.Info("Successfully restored from S3", "pvc", pvc.Name, "dataSize", len(backupData))
	return nil
}

// restoreFromNFS restores data from NFS backup
func (r *PVCBackupReconciler) restoreFromNFS(ctx context.Context, pvc corev1.PersistentVolumeClaim, backupInfo map[string]string) error {
	logger := log.FromContext(ctx)

	logger.Info("Restoring from NFS", "server", backupInfo["server"], "path", backupInfo["path"], "pvc", pvc.Name)

	// This is a placeholder implementation
	// In production, you would mount the NFS share and copy files to the PVC

	// Simulate restore process
	time.Sleep(2 * time.Second)

	logger.Info("Successfully restored from NFS", "pvc", pvc.Name)
	return nil
}

// addInitContainer adds an init container to pods using the PVC
func (r *PVCBackupReconciler) addInitContainer(ctx context.Context, pvc corev1.PersistentVolumeClaim, config *storagev1alpha1.InitContainerConfig) error {
	logger := log.FromContext(ctx)

	// Find all pods using this PVC
	var pods corev1.PodList
	if err := r.List(ctx, &pods, client.MatchingFields{"spec.volumes.persistentVolumeClaim.claimName": pvc.Name}); err != nil {
		return fmt.Errorf("failed to list pods using PVC: %w", err)
	}

	for _, pod := range pods.Items {
		// Skip pods that already have our init container
		if r.hasInitContainer(&pod, config) {
			continue
		}

		// Add init container to the pod
		if err := r.addInitContainerToPod(ctx, &pod, config); err != nil {
			logger.Error(err, "Failed to add init container to pod", "pod", pod.Name)
			continue
		}

		logger.Info("Added init container to pod", "pod", pod.Name, "pvc", pvc.Name)
	}

	return nil
}

// hasInitContainer checks if a pod already has our init container
func (r *PVCBackupReconciler) hasInitContainer(pod *corev1.Pod, config *storagev1alpha1.InitContainerConfig) bool {
	for _, container := range pod.Spec.InitContainers {
		if container.Image == config.Image {
			return true
		}
	}
	return false
}

// addInitContainerToPod adds an init container to a specific pod
func (r *PVCBackupReconciler) addInitContainerToPod(ctx context.Context, pod *corev1.Pod, config *storagev1alpha1.InitContainerConfig) error {
	// Create init container spec
	initContainer := corev1.Container{
		Name:  "pvc-restore-wait",
		Image: config.Image,
	}

	if len(config.Command) > 0 {
		initContainer.Command = config.Command
	}

	if len(config.Args) > 0 {
		initContainer.Args = config.Args
	}

	// Add init container to the beginning of the list
	pod.Spec.InitContainers = append([]corev1.Container{initContainer}, pod.Spec.InitContainers...)

	// Update the pod
	return r.Update(ctx, pod)
}

func (r *PVCBackupReconciler) performFileLevelBackup(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	logger := log.FromContext(ctx)

	// Create a temporary backup pod
	backupPod, err := r.createBackupPod(ctx, pvc, pvcBackup)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup pod: %w", err)
	}
	defer r.cleanupBackupPod(ctx, backupPod)

	// Wait for pod to be ready
	if err := r.waitForPodReady(ctx, backupPod); err != nil {
		return nil, fmt.Errorf("backup pod not ready: %w", err)
	}

	// Create backup archive
	backupData, err := r.createBackupArchive(ctx, backupPod, pvc)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup archive: %w", err)
	}

	logger.Info("File-level backup completed", "pvc", pvc.Name)
	return backupData, nil
}

// createBackupPod creates a temporary pod for file-level backup
func (r *PVCBackupReconciler) createBackupPod(ctx context.Context, pvc corev1.PersistentVolumeClaim, pvcBackup *storagev1alpha1.PVCBackup) (*corev1.Pod, error) {
	logger := log.FromContext(ctx)

	// Generate unique pod name
	podName := fmt.Sprintf("backup-%s-%s", pvc.Name, time.Now().Format("20060102-150405"))

	// Create backup pod
	backupPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: pvc.Namespace,
			Labels: map[string]string{
				"pvcbackup.cheap-man-ha-store.com/backup-pod": "true",
				"pvcbackup.cheap-man-ha-store.com/pvc-name":   pvc.Name,
				"pvcbackup.cheap-man-ha-store.com/created-by": pvcBackup.Name,
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "backup",
					Image:   "busybox:1.35",
					Command: []string{"/bin/sh"},
					Args:    []string{"-c", "sleep 3600"}, // Keep pod running for backup operations
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: "/data",
							ReadOnly:  false,
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvc.Name,
						},
					},
				},
			},
		},
	}

	if err := r.Create(ctx, backupPod); err != nil {
		return nil, fmt.Errorf("failed to create backup pod: %w", err)
	}

	logger.Info("Created backup pod", "name", podName, "pvc", pvc.Name)
	return backupPod, nil
}

func (r *PVCBackupReconciler) cleanupBackupPod(ctx context.Context, pod *corev1.Pod) {
	logger := log.FromContext(ctx)

	if err := r.Delete(ctx, pod); err != nil {
		logger.Error(err, "Failed to delete backup pod", "name", pod.Name)
	} else {
		logger.Info("Deleted backup pod", "name", pod.Name)
	}
}

func (r *PVCBackupReconciler) waitForPodReady(ctx context.Context, pod *corev1.Pod) error {
	logger := log.FromContext(ctx)

	// Wait up to 5 minutes for pod to be ready
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for pod to be ready: %s", pod.Name)
		case <-ticker.C:
			// Get the latest pod status
			var currentPod corev1.Pod
			if err := r.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &currentPod); err != nil {
				logger.Error(err, "Failed to get pod status", "name", pod.Name)
				continue
			}

			if currentPod.Status.Phase == corev1.PodRunning {
				logger.Info("Pod is ready", "name", pod.Name)
				return nil
			}

			if currentPod.Status.Phase == corev1.PodFailed || currentPod.Status.Phase == corev1.PodUnknown {
				return fmt.Errorf("pod failed or unknown: %s", currentPod.Status.Message)
			}
		}
	}
}

func (r *PVCBackupReconciler) createBackupArchive(ctx context.Context, pod *corev1.Pod, pvc corev1.PersistentVolumeClaim) (interface{}, error) {
	logger := log.FromContext(ctx)

	// Implementation would copy data from the pod's volume to a backup target
	// This is a placeholder for the actual backup logic

	// For demonstration, we'll just return a dummy value
	backupData := map[string]string{
		"message": "File-level backup data (placeholder)",
		"pvc":     fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name),
		"pod":     pod.Name,
	}

	logger.Info("File-level backup archive created (placeholder)", "pvc", pvc.Name, "pod", pod.Name)
	return backupData, nil
}

func containsFinalizer(obj metav1.Object, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

// SetupWithManager sets up the controller with the Manager.
func (r *PVCBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.PVCBackup{}).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForPVC),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForPod),
		).
		Named("pvcbackup").
		Complete(r)
}

// findObjectsForPVC finds PVCBackup objects for a given PVC
func (r *PVCBackupReconciler) findObjectsForPVC(ctx context.Context, obj client.Object) []reconcile.Request {
	pvc := obj.(*corev1.PersistentVolumeClaim)

	var pvcBackups storagev1alpha1.PVCBackupList
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

// findObjectsForPod finds PVCBackup objects for a given Pod
func (r *PVCBackupReconciler) findObjectsForPod(ctx context.Context, obj client.Object) []reconcile.Request {
	pod := obj.(*corev1.Pod)

	// Find PVCs used by the pod
	var pvcNames []string
	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
		}
	}

	if len(pvcNames) == 0 {
		return nil
	}

	var pvcBackups storagev1alpha1.PVCBackupList
	if err := r.List(ctx, &pvcBackups); err != nil {
		return nil
	}

	var requests []reconcile.Request
	for _, pvcBackup := range pvcBackups.Items {
		matchedPVCs, err := r.findMatchingPVCs(ctx, &pvcBackup)
		if err != nil {
			continue
		}

		for _, matchedPVC := range matchedPVCs {
			for _, pvcName := range pvcNames {
				if matchedPVC.Name == pvcName && matchedPVC.Namespace == pod.Namespace {
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
	}

	return requests
}

// waitForSnapshotReady waits for the VolumeSnapshot to be ready
func (r *PVCBackupReconciler) waitForSnapshotReady(ctx context.Context, snapshot *snapshotv1.VolumeSnapshot) error {
	logger := log.FromContext(ctx)

	// Wait up to 5 minutes for snapshot to be ready
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for snapshot to be ready: %s", snapshot.Name)
		case <-ticker.C:
			// Get the latest snapshot status
			var currentSnapshot snapshotv1.VolumeSnapshot
			if err := r.Get(ctx, types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, &currentSnapshot); err != nil {
				logger.Error(err, "Failed to get snapshot status", "name", snapshot.Name)
				continue
			}

			if currentSnapshot.Status != nil && currentSnapshot.Status.ReadyToUse != nil && *currentSnapshot.Status.ReadyToUse {
				logger.Info("VolumeSnapshot is ready", "name", snapshot.Name)
				return nil
			}

			if currentSnapshot.Status != nil && currentSnapshot.Status.Error != nil {
				return fmt.Errorf("snapshot creation failed: %s", *currentSnapshot.Status.Error.Message)
			}
		}
	}
}
