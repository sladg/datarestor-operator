package controller

import (
	"context"
	"fmt"
	"time"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (r *PVCBackupReconciler) isPVCNew(pvc corev1.PersistentVolumeClaim) bool {
	return time.Since(pvc.CreationTimestamp.Time) < 5*time.Minute
}

func (r *PVCBackupReconciler) shouldRestorePVC(pvc corev1.PersistentVolumeClaim) bool {
	return pvc.Labels["backup.restore"] == "true"
}

func (r *PVCBackupReconciler) updateManagedPVCsStatus(ctx context.Context, pvcBackup *storagev1alpha1.PVCBackup, pvcs []corev1.PersistentVolumeClaim) error {
	logger := LoggerFrom(ctx, "status").
		WithValues("name", pvcBackup.Name)

	logger.Starting("update managed PVCs")

	var pvcNames []string
	for _, pvc := range pvcs {
		pvcNames = append(pvcNames, fmt.Sprintf("%s/%s", pvc.Namespace, pvc.Name))
	}
	pvcBackup.Status.ManagedPVCs = pvcNames

	if err := r.Status().Update(ctx, pvcBackup); err != nil {
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
	logger := LoggerFrom(ctx, "scale").
		WithValues(
			"pod", pod.Name,
			"namespace", pod.Namespace,
			"owner", ownerRef.Name,
			"kind", ownerRef.Kind,
		)

	logger.Starting("scale down")

	var err error
	switch ownerRef.Kind {
	case "Deployment":
		err = r.scaleDeployment(ctx, ownerRef, pod.Namespace, 0)
	case "StatefulSet":
		err = r.scaleStatefulSet(ctx, ownerRef, pod.Namespace, 0)
	case "ReplicaSet":
		err = r.scaleReplicaSet(ctx, ownerRef, pod.Namespace, 0)
	default:
		err = fmt.Errorf("unsupported owner kind: %s", ownerRef.Kind)
	}

	if err != nil {
		logger.Failed("scale down", err)
		return err
	}

	logger.Completed("scale down")
	return nil
}

// scaleUpResource scales up a resource to its original replica count
func (r *PVCBackupReconciler) scaleUpResource(ctx context.Context, ownerRef *metav1.OwnerReference, pod *corev1.Pod) error {
	logger := LoggerFrom(ctx, "scale").
		WithValues(
			"pod", pod.Name,
			"namespace", pod.Namespace,
			"owner", ownerRef.Name,
			"kind", ownerRef.Kind,
		)

	logger.Starting("scale up")

	// For now, scale to 1 replica
	// In production, you might want to store the original replica count
	var err error
	switch ownerRef.Kind {
	case "Deployment":
		err = r.scaleDeployment(ctx, ownerRef, pod.Namespace, 1)
	case "StatefulSet":
		err = r.scaleStatefulSet(ctx, ownerRef, pod.Namespace, 1)
	case "ReplicaSet":
		err = r.scaleReplicaSet(ctx, ownerRef, pod.Namespace, 1)
	default:
		err = fmt.Errorf("unsupported owner kind: %s", ownerRef.Kind)
	}

	if err != nil {
		logger.Failed("scale up", err)
		return err
	}

	logger.Completed("scale up")
	return nil
}

// scaleDeployment scales a deployment to the specified replica count
func (r *PVCBackupReconciler) scaleDeployment(ctx context.Context, ownerRef *metav1.OwnerReference, namespace string, replicas int32) error {
	logger := LoggerFrom(ctx, "scale").
		WithValues(
			"kind", "Deployment",
			"name", ownerRef.Name,
			"namespace", namespace,
			"replicas", replicas,
		)

	logger.Starting("scale deployment")

	var deployment appsv1.Deployment
	if err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: namespace}, &deployment); err != nil {
		logger.Failed("get deployment", err)
		return err
	}

	deployment.Spec.Replicas = &replicas
	if err := r.Update(ctx, &deployment); err != nil {
		logger.Failed("update deployment", err)
		return err
	}

	logger.Completed("scale deployment")
	return nil
}

// scaleStatefulSet scales a statefulset to the specified replica count
func (r *PVCBackupReconciler) scaleStatefulSet(ctx context.Context, ownerRef *metav1.OwnerReference, namespace string, replicas int32) error {
	logger := LoggerFrom(ctx, "scale").
		WithValues(
			"kind", "StatefulSet",
			"name", ownerRef.Name,
			"namespace", namespace,
			"replicas", replicas,
		)

	logger.Starting("scale statefulset")

	var statefulSet appsv1.StatefulSet
	if err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: namespace}, &statefulSet); err != nil {
		logger.Failed("get statefulset", err)
		return err
	}

	statefulSet.Spec.Replicas = &replicas
	if err := r.Update(ctx, &statefulSet); err != nil {
		logger.Failed("update statefulset", err)
		return err
	}

	logger.Completed("scale statefulset")
	return nil
}

// scaleReplicaSet scales a replicaset to the specified replica count
func (r *PVCBackupReconciler) scaleReplicaSet(ctx context.Context, ownerRef *metav1.OwnerReference, namespace string, replicas int32) error {
	logger := LoggerFrom(ctx, "scale").
		WithValues(
			"kind", "ReplicaSet",
			"name", ownerRef.Name,
			"namespace", namespace,
			"replicas", replicas,
		)

	logger.Starting("scale replicaset")

	var replicaSet appsv1.ReplicaSet
	if err := r.Get(ctx, types.NamespacedName{Name: ownerRef.Name, Namespace: namespace}, &replicaSet); err != nil {
		logger.Failed("get replicaset", err)
		return err
	}

	replicaSet.Spec.Replicas = &replicas
	if err := r.Update(ctx, &replicaSet); err != nil {
		logger.Failed("update replicaset", err)
		return err
	}

	logger.Completed("scale replicaset")
	return nil
}
