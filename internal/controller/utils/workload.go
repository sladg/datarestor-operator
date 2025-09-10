package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
)

// workloadInfo is a temporary struct used for (un)marshalling replica counts.
type workloadInfo struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Replicas int32  `json:"replicas"`
}

// getWorkloadSpecReplicas returns desired replicas from Spec, defaulting to 1 when nil.
func getWorkloadSpecReplicas(obj client.Object) int32 {
	switch w := obj.(type) {
	case *appsv1.Deployment:
		if w.Spec.Replicas == nil {
			return 1
		}
		return *w.Spec.Replicas
	case *appsv1.StatefulSet:
		if w.Spec.Replicas == nil {
			return 1
		}
		return *w.Spec.Replicas
	default:
		return 0
	}
}

// getWorkloadReadyReplicas returns ready replicas from Status.
func getWorkloadReadyReplicas(obj client.Object) int32 {
	switch w := obj.(type) {
	case *appsv1.Deployment:
		return w.Status.ReadyReplicas
	case *appsv1.StatefulSet:
		return w.Status.ReadyReplicas
	default:
		return 0
	}
}

// scaleSingleWorkload patches replicas for a single workload.
func scaleSingleWorkload(ctx context.Context, c client.Client, workload client.Object, replicas int32, logger *zap.SugaredLogger) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"replicas":%d}}`, replicas))
	if err := c.Patch(ctx, workload, client.RawPatch(types.MergePatchType, patch)); err != nil {
		logger.Errorw("Failed to scale workload", err, "workload", workload.GetName())
		return err
	}
	logger.Infow("Scaled workload", "workload", workload.GetName(), "replicas", replicas)
	return nil
}

// FindWorkloadsForPVC returns top-level scalable workloads (Deployment/StatefulSet) mounting the PVC.
func FindWorkloadsForPVC(ctx context.Context, c client.Client, pvc corev1.ObjectReference, logger *zap.SugaredLogger) ([]client.Object, error) {
	logger = logger.Named("workload-scaler/find").With("pvc", pvc.Name, "namespace", pvc.Namespace)

	podList := &corev1.PodList{}
	if err := c.List(ctx, podList, client.InNamespace(pvc.Namespace)); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	mountingPods := make([]corev1.Pod, 0, len(podList.Items))
	for _, pod := range podList.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName == pvc.Name {
				mountingPods = append(mountingPods, pod)
				break
			}
		}
	}

	if len(mountingPods) == 0 {
		logger.Debug("No pods found mounting this PVC")
		return nil, nil
	}

	workloads := make([]client.Object, 0, len(mountingPods))
	processedOwners := make(map[types.UID]bool)

	for _, pod := range mountingPods {
		owner := metav1.GetControllerOf(&pod)
		if owner == nil {
			continue
		}

		wl, err := findScalableWorkload(ctx, c, owner, pod.Namespace)
		if err != nil {
			logger.Errorw("Could not find scalable workload for pod", err, "pod", pod.Name)
			continue
		}
		if wl == nil || processedOwners[wl.GetUID()] {
			continue
		}

		processedOwners[wl.GetUID()] = true
		workloads = append(workloads, wl)
	}

	return workloads, nil
}

// ScaleWorkloadsTo scales provided workloads to the desired replicas.
func ScaleWorkloadsTo(ctx context.Context, c client.Client, workloads []client.Object, replicas int32, logger *zap.SugaredLogger) {
	for _, wl := range workloads {
		_ = scaleSingleWorkload(ctx, c, wl, replicas, logger)
	}
}

func GetOriginalReplicasInfo(ctx context.Context, deps *Dependencies, workloads []client.Object) (map[string]workloadInfo, error) {
	originalReplicas := make(map[string]workloadInfo)
	for _, workload := range workloads {
		switch w := workload.(type) {
		case *appsv1.Deployment:
			originalReplicas[string(w.UID)] = workloadInfo{Kind: "Deployment", Name: w.Name, Replicas: getWorkloadSpecReplicas(w)}
		case *appsv1.StatefulSet:
			originalReplicas[string(w.UID)] = workloadInfo{Kind: "StatefulSet", Name: w.Name, Replicas: getWorkloadSpecReplicas(w)}
		}
	}

	return originalReplicas, nil
}

func SetOriginalReplicasAnnotation(ctx context.Context, deps *Dependencies, owner client.Object, originalReplicas map[string]workloadInfo) error {
	jsonData, err := json.Marshal(originalReplicas)
	if err != nil {
		return fmt.Errorf("failed to marshal original replica counts: %w", err)
	}

	annotations := owner.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[constants.AnnOriginalReplicas] = string(jsonData)
	owner.SetAnnotations(annotations)

	return deps.Update(ctx, owner)
}

func LoadOriginalReplicasFromAnnotation(owner client.Object) (map[string]workloadInfo, error) {
	annotations := owner.GetAnnotations()
	jsonData, ok := annotations[constants.AnnOriginalReplicas]
	if !ok || jsonData == "" {
		return nil, nil // No annotation found, not an error.
	}

	originalReplicas := map[string]workloadInfo{}
	if err := json.Unmarshal([]byte(jsonData), &originalReplicas); err != nil {
		return nil, fmt.Errorf("failed to unmarshal original replica counts: %w", err)
	}
	return originalReplicas, nil
}

// RemoveOriginalReplicasAnnotation removes the original replica count annotation from the owner.
func RemoveOriginalReplicasAnnotation(ctx context.Context, deps *Dependencies, owner client.Object) error {
	annotations := owner.GetAnnotations()
	if _, ok := annotations[constants.AnnOriginalReplicas]; !ok {
		return nil // Annotation doesn't exist, nothing to do.
	}

	delete(annotations, constants.AnnOriginalReplicas)
	owner.SetAnnotations(annotations)
	return deps.Update(ctx, owner)
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

// StopPodsInfo summarizes the effective stop-pods policy and its source.
type StopPodsInfo struct {
	Enabled         bool
	Source          string // "spec", "selectors", or "none"
	SelectorIndexes []int  // indexes of selectors that enabled stop-pods (when Source=="selectors")
}

// ShouldStopPods evaluates Config.Spec.StopPods and Selector.StopPods and returns decision details.
func ShouldStopPods(cfg *v1.Config) StopPodsInfo {
	if cfg.Spec.StopPods {
		return StopPodsInfo{Enabled: true, Source: "spec"}
	}

	indexes := make([]int, 0, len(cfg.Spec.Selectors))
	for i, sel := range cfg.Spec.Selectors {
		if sel.StopPods {
			indexes = append(indexes, i)
		}
	}
	if len(indexes) > 0 {
		return StopPodsInfo{Enabled: true, Source: "selectors", SelectorIndexes: indexes}
	}

	return StopPodsInfo{Enabled: false, Source: "none"}
}

// ScaleDownWorkloadsForPVC scales down all workloads mounting the PVC to 0 and stores original replicas on the owner.
func ScaleDownWorkloadsForPVC(ctx context.Context, deps *Dependencies, pvc corev1.ObjectReference, owner client.Object) (bool, error) {
	log := deps.Logger.Named("[ScaleDownWorkloadsForPVC]").With("pvc", pvc.Name, "namespace", pvc.Namespace)

	workloads, err := FindWorkloadsForPVC(ctx, deps.Client, pvc, log)
	if err != nil {
		return false, err
	}
	if len(workloads) == 0 {
		log.Info("No workloads found for PVC, skipping scale down")
		return false, nil
	}
	originalReplicas, err := GetOriginalReplicasInfo(ctx, deps, workloads)
	if err != nil {
		return true, err
	}
	if err := SetOriginalReplicasAnnotation(ctx, deps, owner, originalReplicas); err != nil {
		return true, err
	}

	ScaleWorkloadsTo(ctx, deps.Client, workloads, 0, log)
	return true, nil
}

// ScaleUpWorkloadsForPVC restores replicas from the owner's original-replicas annotation.
func ScaleUpWorkloadsForPVC(ctx context.Context, deps *Dependencies, pvc corev1.ObjectReference, owner client.Object) (bool, error) {
	log := deps.Logger.Named("[ScaleUpWorkloadsForPVC]").With("pvc", pvc.Name, "namespace", pvc.Namespace)

	workloads, err := FindWorkloadsForPVC(ctx, deps.Client, pvc, log)
	if err != nil {
		return false, err
	}
	if len(workloads) == 0 {
		log.Info("No workloads found for PVC, skipping scale up")
		return false, nil
	}
	originalReplicas, err := LoadOriginalReplicasFromAnnotation(owner)
	if err != nil {
		return true, err
	}
	if originalReplicas == nil {
		log.Info("No original replica annotation found, skipping scale up")
		return true, nil
	}
	for _, wl := range workloads {
		info, ok := originalReplicas[string(wl.GetUID())]
		if !ok {
			continue
		}
		_ = scaleSingleWorkload(ctx, deps.Client, wl, info.Replicas, log)
	}
	return true, RemoveOriginalReplicasAnnotation(ctx, deps, owner)
}

// IsScaleDownCompleteForPVC returns true if all workloads mounting the PVC report 0 ready replicas.
func IsScaleDownCompleteForPVC(ctx context.Context, deps *Dependencies, pvc corev1.ObjectReference) (bool, error) {
	log := deps.Logger.Named("workload-scaler/status-down").With("pvc", pvc.Name, "namespace", pvc.Namespace)
	workloads, err := FindWorkloadsForPVC(ctx, deps.Client, pvc, log)
	if err != nil {
		return false, err
	}
	for _, wl := range workloads {
		if getWorkloadReadyReplicas(wl) > 0 {
			return false, nil
		}
	}
	return true, nil
}

// IsScaleUpCompleteForPVC returns true if all annotated workloads have reached their original replicas.
func IsScaleUpCompleteForPVC(ctx context.Context, deps *Dependencies, pvc corev1.ObjectReference, owner client.Object) (bool, error) {
	log := deps.Logger.Named("workload-scaler/status-up").With("pvc", pvc.Name, "namespace", pvc.Namespace)
	originalReplicas, err := LoadOriginalReplicasFromAnnotation(owner)
	if err != nil {
		return false, err
	}
	if originalReplicas == nil {
		return true, nil
	}
	workloads, err := FindWorkloadsForPVC(ctx, deps.Client, pvc, log)
	if err != nil {
		return false, err
	}
	for _, wl := range workloads {
		info, ok := originalReplicas[string(wl.GetUID())]
		if !ok {
			continue
		}
		if getWorkloadReadyReplicas(wl) < info.Replicas {
			return false, nil
		}
	}
	return true, nil
}

// WaitForScaleDownForPVC waits until all workloads report 0 ready replicas or timeout.
func WaitForScaleDownForPVC(ctx context.Context, deps *Dependencies, pvc corev1.ObjectReference, timeout time.Duration, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		done, err := IsScaleDownCompleteForPVC(ctx, deps, pvc)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timed out waiting for pods to terminate for PVC %s", pvc.Name)
}

// WaitForScaleUpForPVC waits until all annotated workloads reach original replicas or timeout.
func WaitForScaleUpForPVC(ctx context.Context, deps *Dependencies, pvc corev1.ObjectReference, owner client.Object, timeout time.Duration, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		done, err := IsScaleUpCompleteForPVC(ctx, deps, pvc, owner)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		time.Sleep(interval)
	}
	return fmt.Errorf("timed out waiting for workloads to scale up for PVC %s", pvc.Name)
}
