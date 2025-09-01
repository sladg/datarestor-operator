package utils

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ScaleWorkloads finds all Deployments and StatefulSets using a given PVC and scales them
// to the specified replica count. It returns a list of the workloads that were scaled.
// A replica count of -1 can be used to just find the workloads without scaling them.
func ScaleWorkloads(ctx context.Context, c client.Client, pvc *corev1.PersistentVolumeClaim, replicas int32, logger *zap.SugaredLogger) ([]client.Object, error) {
	logger = logger.Named("workload-scaler").With("pvc", pvc.Name, "namespace", pvc.Namespace)
	logger.Debugw("Finding/Scaling workloads", "replicas", replicas)

	// 1. Find all pods in the PVC's namespace
	podList := &corev1.PodList{}
	if err := c.List(ctx, podList, client.InNamespace(pvc.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// 2. Filter pods to find those mounting the specific PVC
	var mountingPods []corev1.Pod
	for _, pod := range podList.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				mountingPods = append(mountingPods, pod)
				break // Move to the next pod
			}
		}
	}

	if len(mountingPods) == 0 {
		logger.Debug("No pods found mounting this PVC, nothing to scale.")
		return nil, nil
	}

	// 3. Find the owner workloads (Deployments or StatefulSets) and scale them
	var scaledWorkloads []client.Object
	processedOwners := make(map[types.UID]bool)

	for _, pod := range mountingPods {
		owner := metav1.GetControllerOf(&pod)
		if owner == nil {
			continue
		}

		workload, err := findScalableWorkload(ctx, c, owner, pod.Namespace)
		if err != nil {
			logger.Errorw("Could not find scalable workload for pod", "error", err, "pod", pod.Name)
			continue
		}

		if workload == nil || processedOwners[workload.GetUID()] {
			continue // Not a scalable workload we handle, or already processed
		}

		// Mark as processed
		processedOwners[workload.GetUID()] = true

		// If replicas is -1, we are only finding workloads, not scaling them.
		if replicas != -1 {
			// Patch the workload to the desired replica count
			patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
			if err := c.Patch(ctx, workload, client.RawPatch(types.MergePatchType, patch)); err != nil {
				logger.Errorw("Failed to scale workload", "error", err, "workload", workload.GetName())
				// Continue to try scaling other workloads
				continue
			}
			logger.Infow("Successfully scaled workload", "workload", workload.GetName(), "replicas", replicas)
		}
		scaledWorkloads = append(scaledWorkloads, workload)
	}

	return scaledWorkloads, nil
}

// findScalableWorkload traces owner references from a pod to find the top-level
// Deployment or StatefulSet.
func findScalableWorkload(ctx context.Context, c client.Client, owner *metav1.OwnerReference, namespace string) (client.Object, error) {
	// Handle StatefulSets directly
	if owner.Kind == "StatefulSet" {
		sts := &appsv1.StatefulSet{}
		if err := c.Get(ctx, types.NamespacedName{Name: owner.Name, Namespace: namespace}, sts); err != nil {
			return nil, client.IgnoreNotFound(err)
		}
		return sts, nil
	}

	// Handle Deployments (via ReplicaSets)
	if owner.Kind == "ReplicaSet" {
		rs := &appsv1.ReplicaSet{}
		if err := c.Get(ctx, types.NamespacedName{Name: owner.Name, Namespace: namespace}, rs); err != nil {
			return nil, client.IgnoreNotFound(err)
		}

		// A ReplicaSet without an owner is unusual, but we'll stop here.
		rsOwner := metav1.GetControllerOf(rs)
		if rsOwner == nil || rsOwner.Kind != "Deployment" {
			return nil, nil
		}

		dep := &appsv1.Deployment{}
		if err := c.Get(ctx, types.NamespacedName{Name: rsOwner.Name, Namespace: namespace}, dep); err != nil {
			return nil, client.IgnoreNotFound(err)
		}
		return dep, nil
	}

	// We don't handle other kinds of owners (e.g., DaemonSet, Job)
	return nil, nil
}
