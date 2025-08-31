package utils

import (
	"context"
	"fmt"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// DefaultJobTimeout is the default timeout for job cleanup
	DefaultJobTimeout = 10 * time.Minute
	// DefaultDeletionTimeout is the default timeout for resource deletion
	DefaultDeletionTimeout = 5 * time.Minute
	// MaxJobFailures is set to 1 - jobs should not be restarted, if they fail, they fail
	MaxJobFailures = 1
)

// ShouldForceCleanupJob determines if a job should be force cleaned up based on:
// - Running time exceeding DefaultJobTimeout
// - Failed attempts exceeding MaxJobFailures
// - Resource being in deletion state for DefaultDeletionTimeout
func ShouldForceCleanupJob(job *batchv1.Job, deletionTimestamp *time.Time) bool {
	if job.Status.StartTime != nil {
		runningTime := time.Since(job.Status.StartTime.Time)
		if runningTime > DefaultJobTimeout {
			return true
		}
	}

	if job.Status.Failed >= MaxJobFailures {
		return true
	}

	if deletionTimestamp != nil {
		deletionTime := time.Since(*deletionTimestamp)
		if deletionTime > DefaultDeletionTimeout {
			return true
		}
	}

	return false
}

// AddFinalizer adds a finalizer to a resource if it doesn't exist
func AddFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	if !controllerutil.ContainsFinalizer(obj, finalizer) {
		controllerutil.AddFinalizer(obj, finalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to add finalizer: %w", err)
		}
	}
	return nil
}

// RemoveFinalizer removes a finalizer from a resource if it exists
func RemoveFinalizer(ctx context.Context, c client.Client, obj client.Object, finalizer string) error {
	if controllerutil.ContainsFinalizer(obj, finalizer) {
		controllerutil.RemoveFinalizer(obj, finalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("failed to remove finalizer: %w", err)
		}
	}
	return nil
}

// FindMatchingPVCs finds PVCs matching the given labels and namespaces
func FindMatchingPVCs(ctx context.Context, c client.Client, labels map[string]string, namespaces []string) ([]corev1.PersistentVolumeClaim, error) {
	var allPVCs []corev1.PersistentVolumeClaim

	// If no namespaces provided, search in all namespaces
	if len(namespaces) == 0 {
		var pvcList corev1.PersistentVolumeClaimList
		if err := c.List(ctx, &pvcList, client.MatchingLabels(labels)); err != nil {
			return nil, fmt.Errorf("failed to list PVCs: %w", err)
		}
		allPVCs = pvcList.Items
	} else {
		// Search in specified namespaces
		for _, ns := range namespaces {
			var pvcList corev1.PersistentVolumeClaimList
			if err := c.List(ctx, &pvcList, client.InNamespace(ns), client.MatchingLabels(labels)); err != nil {
				return nil, fmt.Errorf("failed to list PVCs in namespace %s: %w", ns, err)
			}
			allPVCs = append(allPVCs, pvcList.Items...)
		}
	}

	return allPVCs, nil
}
