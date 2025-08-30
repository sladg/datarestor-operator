package controller

import (
	"context"
	"fmt"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *BackupConfigReconciler) updateManagedPVCsStatus(ctx context.Context, backupConfig *backupv1alpha1.BackupConfig, pvcs []corev1.PersistentVolumeClaim) error {
	logger := LoggerFrom(ctx, "status").
		WithValues("name", backupConfig.Name)

	logger.Starting("update managed PVCs")

	var pvcNames []string
	for _, pvc := range pvcs {
		pvcNames = append(pvcNames, fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name))
	}
	backupConfig.Status.ManagedPVCs = pvcNames

	if err := r.Status().Update(ctx, backupConfig); err != nil {
		logger.Failed("update status", err)
		return err
	}

	logger.WithValues("pvcs", pvcNames).Debug("Updated managed PVCs")
	logger.Completed("update managed PVCs")
	return nil
}

func containsFinalizer(obj metav1.Object, finalizer string) bool {
	for _, f := range obj.GetFinalizers() {
		if f == finalizer {
			return true
		}
	}
	return false
}

// Generic replica management functions

// storeOriginalReplicas stores the original replica count in resource annotations
func storeOriginalReplicasGeneric(ctx context.Context, client client.Client, obj client.Object, originalReplicas int32) error {
	// Get current annotations
	var annotations map[string]string
	if obj.GetAnnotations() == nil {
		annotations = make(map[string]string)
	} else {
		annotations = obj.GetAnnotations()
	}

	annotations[OriginalReplicasAnnotation] = fmt.Sprintf("%d", originalReplicas)
	obj.SetAnnotations(annotations)

	return client.Update(ctx, obj)
}

// getOriginalReplicasGeneric retrieves the original replica count from resource annotations
func getOriginalReplicasGeneric(obj client.Object) (int32, error) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return 0, fmt.Errorf("no annotations found")
	}

	replicaStr, exists := annotations[OriginalReplicasAnnotation]
	if !exists {
		return 0, fmt.Errorf("original replicas annotation not found")
	}

	var result int32
	if _, err := fmt.Sscanf(replicaStr, "%d", &result); err != nil {
		return 0, fmt.Errorf("failed to parse original replicas: %w", err)
	}

	return result, nil
}

// clearOriginalReplicasGeneric removes the original replica count annotation from resource
func clearOriginalReplicasGeneric(ctx context.Context, client client.Client, obj client.Object) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	delete(annotations, OriginalReplicasAnnotation)
	obj.SetAnnotations(annotations)

	return client.Update(ctx, obj)
}

// scaleResourceGeneric scales a deployment or statefulset with replica management
func scaleResourceGeneric(ctx context.Context, client client.Client, obj client.Object, pvcName string, enable bool) error {
	logger := LoggerFrom(ctx, "scale-resource").
		WithValues("resource", obj.GetName(), "namespace", obj.GetNamespace(), "enable", enable)

	var currentReplicas *int32
	var targetReplicas int32

	// Type switch to handle different resource types
	switch resource := obj.(type) {
	case *appsv1.Deployment:
		currentReplicas = resource.Spec.Replicas
		if !enable {
			// Disabling: Store original replica count and scale to 0
			if currentReplicas != nil && *currentReplicas > 0 {
				if err := storeOriginalReplicasGeneric(ctx, client, obj, *currentReplicas); err != nil {
					logger.Failed("store original replicas", err)
					return err
				}
			}
			targetReplicas = 0
		} else {
			// Enabling: Try to restore original replica count, fallback to 1
			if originalReplicas, err := getOriginalReplicasGeneric(obj); err == nil && originalReplicas > 0 {
				targetReplicas = originalReplicas
				logger.WithValues("original_replicas", originalReplicas).Debug("Restoring original replica count")
			} else {
				targetReplicas = 1
			}
		}
		resource.Spec.Replicas = &targetReplicas

	case *appsv1.StatefulSet:
		currentReplicas = resource.Spec.Replicas
		if !enable {
			// Disabling: Store original replica count and scale to 0
			if currentReplicas != nil && *currentReplicas > 0 {
				if err := storeOriginalReplicasGeneric(ctx, client, obj, *currentReplicas); err != nil {
					logger.Failed("store original replicas", err)
					return err
				}
			}
			targetReplicas = 0
		} else {
			// Enabling: Try to restore original replica count, fallback to 1
			if originalReplicas, err := getOriginalReplicasGeneric(obj); err == nil && originalReplicas > 0 {
				targetReplicas = originalReplicas
				logger.WithValues("original_replicas", originalReplicas).Debug("Restoring original replica count")
			} else {
				targetReplicas = 1
			}
		}
		resource.Spec.Replicas = &targetReplicas

	default:
		return fmt.Errorf("unsupported resource type: %T", obj)
	}

	// Update the resource
	if err := client.Update(ctx, obj); err != nil {
		logger.Failed("update resource", err)
		return err
	}

	// Clean up annotation when enabling
	if enable {
		if err := clearOriginalReplicasGeneric(ctx, client, obj); err != nil {
			logger.Failed("clear original replicas annotation", err)
			// Continue anyway - this is not critical
		}
	}

	logger.WithValues("target_replicas", targetReplicas).Debug("Successfully scaled resource")
	return nil
}

// enableDeploymentsUsingPVC enables/disables all deployments/statefulsets using the specified PVC
func enableDeploymentsUsingPVC(ctx context.Context, client client.Client, pvc corev1.PersistentVolumeClaim, enable bool) error {
	logger := LoggerFrom(ctx, "deployment-scale").
		WithValues("pvc", pvc.Name, "namespace", pvc.Namespace, "enable", enable)

	action := "disable"
	if enable {
		action = "enable"
	}
	logger.Starting(fmt.Sprintf("%s deployments using PVC", action))

	// Scale Deployments using this PVC
	if err := scaleDeploymentsUsingPVC(ctx, client, pvc, enable); err != nil {
		logger.Failed(fmt.Sprintf("%s deployments", action), err)
		return err
	}

	// Scale StatefulSets using this PVC
	if err := scaleStatefulSetsUsingPVC(ctx, client, pvc, enable); err != nil {
		logger.Failed(fmt.Sprintf("%s statefulsets", action), err)
		return err
	}

	logger.Completed(fmt.Sprintf("%s deployments using PVC", action))
	return nil
}

// scaleDeploymentsUsingPVC finds all deployments using the PVC and enables/disables them
func scaleDeploymentsUsingPVC(ctx context.Context, client client.Client, pvc corev1.PersistentVolumeClaim, enable bool) error {
	logger := LoggerFrom(ctx, "deployment-scale").
		WithValues("pvc", pvc.Name, "namespace", pvc.Namespace, "enable", enable)

	// List all deployments
	var deployments appsv1.DeploymentList
	if err := client.List(ctx, &deployments); err != nil {
		logger.Failed("list deployments", err)
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	scaledCount := 0
	for _, deployment := range deployments.Items {
		// Check if deployment is in the same namespace and uses this PVC
		if deployment.Namespace == pvc.Namespace && deploymentUsesPVC(deployment, pvc.Name) {
			if err := scaleResourceGeneric(ctx, client, &deployment, pvc.Name, enable); err != nil {
				logger.WithValues("deployment", deployment.Name).Failed("scale deployment", err)
				continue
			}
			scaledCount++
		}
	}

	logger.WithValues("scaled_count", scaledCount).Debug("Scaled deployments")
	return nil
}

// scaleStatefulSetsUsingPVC finds all statefulsets using the PVC and enables/disables them
func scaleStatefulSetsUsingPVC(ctx context.Context, client client.Client, pvc corev1.PersistentVolumeClaim, enable bool) error {
	logger := LoggerFrom(ctx, "statefulset-scale").
		WithValues("pvc", pvc.Name, "namespace", pvc.Namespace, "enable", enable)

	// List all statefulsets
	var statefulSets appsv1.StatefulSetList
	if err := client.List(ctx, &statefulSets); err != nil {
		logger.Failed("list statefulsets", err)
		return fmt.Errorf("failed to list statefulsets: %w", err)
	}

	scaledCount := 0
	for _, statefulSet := range statefulSets.Items {
		// Check if statefulset is in the same namespace and uses this PVC
		if statefulSet.Namespace == pvc.Namespace && statefulSetUsesPVC(statefulSet, pvc.Name) {
			if err := scaleResourceGeneric(ctx, client, &statefulSet, pvc.Name, enable); err != nil {
				logger.WithValues("statefulset", statefulSet.Name).Failed("scale statefulset", err)
				continue
			}
			scaledCount++
		}
	}

	logger.WithValues("scaled_count", scaledCount).Debug("Scaled statefulsets")
	return nil
}

// deploymentUsesPVC checks if a deployment uses the specified PVC
func deploymentUsesPVC(deployment appsv1.Deployment, pvcName string) bool {
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
			return true
		}
	}
	return false
}

// statefulSetUsesPVC checks if a statefulset uses the specified PVC
func statefulSetUsesPVC(statefulSet appsv1.StatefulSet, pvcName string) bool {
	// Check volumes in the pod template
	for _, volume := range statefulSet.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvcName {
			return true
		}
	}

	// Check volume claim templates
	for _, vct := range statefulSet.Spec.VolumeClaimTemplates {
		if vct.Name == pvcName {
			return true
		}
	}

	return false
}

// JobStatusOptions holds options for updating job status
type JobStatusOptions struct {
	Phase             string
	Error             string
	RestoreStatus     string // Only used for RestoreJob
	SetStartTime      bool
	SetCompletionTime bool
}

// UpdateJobStatus updates the status of a BackupJob or RestoreJob with the given options
func UpdateJobStatus(
	ctx context.Context,
	client client.Client,
	obj client.Object,
	options JobStatusOptions,
) (ctrl.Result, error) {
	logger := LoggerFrom(ctx, "status-update").
		WithValues("type", fmt.Sprintf("%T", obj))

	now := metav1.Now()

	switch v := obj.(type) {
	case *backupv1alpha1.BackupJob:
		v.Status.Phase = options.Phase
		if options.Error != "" {
			v.Status.Error = options.Error
		} else {
			v.Status.Error = ""
		}
		if options.SetStartTime {
			v.Status.StartTime = &now
		}
		if options.SetCompletionTime {
			v.Status.CompletionTime = &now
		}

	case *backupv1alpha1.RestoreJob:
		v.Status.Phase = options.Phase
		if options.Error != "" {
			v.Status.Error = options.Error
		} else {
			v.Status.Error = ""
		}
		if options.RestoreStatus != "" {
			v.Status.RestoreStatus = options.RestoreStatus
		}
		if options.SetStartTime {
			v.Status.StartTime = &now
		}
		if options.SetCompletionTime {
			v.Status.CompletionTime = &now
		}
	}

	if err := client.Status().Update(ctx, obj); err != nil {
		logger.Failed("update status", err)
		return ctrl.Result{}, err
	}

	// Return error for failed status if one was provided
	if options.Phase == "Failed" && options.Error != "" {
		return ctrl.Result{}, fmt.Errorf("%s", options.Error)
	}

	return ctrl.Result{}, nil
}
