package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ContainsFinalizer checks if a finalizer is present on the object
func ContainsFinalizer(obj client.Object, finalizer string) bool {
	return controllerutil.ContainsFinalizer(obj, finalizer)
}

func ContainsFinalizerWithRef(ctx context.Context, deps *Dependencies, obj corev1.ObjectReference, finalizer string) bool {
	// Get the PVC directly since we know it's a PVC
	var pvc corev1.PersistentVolumeClaim
	err := deps.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, &pvc)
	if err != nil {
		return false
	}
	return ContainsFinalizer(&pvc, finalizer)
}

// AddFinalizer adds a finalizer to a resource if it doesn't exist
func AddFinalizer(ctx context.Context, deps *Dependencies, obj client.Object, finalizer string) error {
	if ContainsFinalizer(obj, finalizer) {
		return nil
	}
	controllerutil.AddFinalizer(obj, finalizer)
	if err := UpdateObjectWithRetry(ctx, deps, obj); err != nil {
		return fmt.Errorf("failed to add finalizer: %w", err)
	}
	return nil
}

func AddFinalizerWithRef(ctx context.Context, deps *Dependencies, obj corev1.ObjectReference, finalizer string) error {
	// Get the PVC directly since we know it's a PVC
	var pvc corev1.PersistentVolumeClaim
	err := deps.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, &pvc)
	if err != nil {
		return fmt.Errorf("failed to get PVC: %w", err)
	}

	return AddFinalizer(ctx, deps, &pvc, finalizer)
}

func RemoveFinalizer(ctx context.Context, deps *Dependencies, obj client.Object, finalizer string) error {
	if !ContainsFinalizer(obj, finalizer) {
		deps.Logger.Debugw("Finalizer not found", "finalizer", finalizer, "obj", obj.GetName())
		return nil
	}
	controllerutil.RemoveFinalizer(obj, finalizer)
	if err := UpdateObjectWithRetry(ctx, deps, obj); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	deps.Logger.Debugw("Finalizer removed", "finalizer", finalizer, "obj", obj.GetName())
	return nil
}

func RemoveFinalizerWithRef(ctx context.Context, deps *Dependencies, obj corev1.ObjectReference, finalizer string) error {
	// Get the PVC directly since we know it's a PVC
	var pvc corev1.PersistentVolumeClaim
	err := deps.Get(ctx, client.ObjectKey{Name: obj.Name, Namespace: obj.Namespace}, &pvc)
	if err != nil {
		return fmt.Errorf("failed to get PVC: %w", err)
	}
	return RemoveFinalizer(ctx, deps, &pvc, finalizer)
}
