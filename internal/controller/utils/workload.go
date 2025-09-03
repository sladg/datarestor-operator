package utils

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/sladg/datarestor-operator/internal/constants"
)

// ManageWorkloadScaleForPVC handles scaling workloads up or down for a given PVC.
// It manages finalizers and annotations on the owner object.
func ManageWorkloadScaleForPVC(ctx context.Context, deps *Dependencies, pvc *corev1.PersistentVolumeClaim, owner client.Object, scaleDown bool) error {
	log := deps.Logger.Named("workload-scaler").With("pvc", pvc.Name, "namespace", pvc.Namespace)

	workloads, err := ScaleWorkloads(ctx, deps.Client, pvc, -1, log)
	if err != nil {
		return fmt.Errorf("failed to find workloads for PVC %s: %w", pvc.Name, err)
	}
	if len(workloads) == 0 {
		log.Info("No workloads found for PVC, skipping scaling")
		return nil
	}

	if scaleDown {
		// Scale Down Logic
		if err := StoreOriginalReplicasInAnnotation(ctx, deps, owner, workloads); err != nil {
			return fmt.Errorf("failed to store original replica annotation: %w", err)
		}

		for _, workload := range workloads {
			if err := AddFinalizer(ctx, deps, workload, constants.WorkloadFinalizer); err != nil {
				log.Errorw("Failed to add finalizer to workload", "error", err, "workload", workload.GetName())
			}
			patch := []byte(`{"spec":{"replicas":0}}`)
			if err := deps.Client.Patch(ctx, workload, client.RawPatch(types.MergePatchType, patch)); err != nil {
				log.Errorw("Failed to scale down workload", "error", err, "workload", workload.GetName())
			}
		}

		return checkPodsTerminated(ctx, deps, pvc)

	} else {
		// Scale Up Logic
		originalReplicas, err := LoadOriginalReplicasFromAnnotation(owner)
		if err != nil {
			return fmt.Errorf("failed to load original replica annotation: %w", err)
		}
		if originalReplicas == nil {
			log.Info("No original replica count annotation found, skipping scale up")
			return nil
		}

		for _, wl := range workloads {
			uid := string(wl.GetUID())
			info, exists := originalReplicas[uid]
			if !exists {
				log.Warnw("Workload was not in the original scale-down list, skipping", "workload", wl.GetName())
				continue
			}

			patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, info.Replicas))
			if err := deps.Client.Patch(ctx, wl, client.RawPatch(types.MergePatchType, patch)); err != nil {
				log.Errorw("Failed to restore replica count", "workload", wl.GetName(), "error", err)
				continue
			}
			log.Infow("Scaled up workload", "workload", wl.GetName(), "replicas", info.Replicas)

			if err := RemoveFinalizer(ctx, deps, wl, constants.WorkloadFinalizer); err != nil {
				log.Errorw("Failed to remove finalizer from workload", "error", err, "workload", wl.GetName())
			}
		}

		return RemoveOriginalReplicasAnnotation(ctx, deps, owner)
	}
}

// checkPodsTerminated waits for all pods using a PVC to be terminated.
func checkPodsTerminated(ctx context.Context, deps *Dependencies, pvc *corev1.PersistentVolumeClaim) error {
	log := deps.Logger.Named("check-pods-terminated")
	for i := 0; i < 30; i++ { // Poll for a max of 5 minutes (30 * 10s)
		workloads, err := ScaleWorkloads(ctx, deps.Client, pvc, -1, log) // -1 means don't scale, just find
		if err != nil {
			return err
		}

		allTerminated := true
		for _, workload := range workloads {
			switch w := workload.(type) {
			case *appsv1.Deployment:
				if w.Status.ReadyReplicas > 0 {
					allTerminated = false
					break
				}
			case *appsv1.StatefulSet:
				if w.Status.ReadyReplicas > 0 {
					allTerminated = false
					break
				}
			}
		}

		if allTerminated {
			log.Info("All pods using PVC have been terminated")
			return nil
		}

		log.Debug("Waiting for pods to terminate...")
		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("timed out waiting for pods to terminate for PVC %s", pvc.Name)
}

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
