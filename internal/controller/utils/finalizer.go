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
	objObj, err := GetResource[client.Object](ctx, deps.Client, obj.Namespace, obj.Name)
	if err != nil {
		return false
	}
	return ContainsFinalizer(objObj, finalizer)
}

// AddFinalizer adds a finalizer to a resource if it doesn't exist
func AddFinalizer(ctx context.Context, deps *Dependencies, obj client.Object, finalizer string) error {
	if ContainsFinalizer(obj, finalizer) {
		return nil
	}
	controllerutil.AddFinalizer(obj, finalizer)
	if err := deps.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to add finalizer: %w", err)
	}
	return nil
}

func AddFinalizerWithRef(ctx context.Context, deps *Dependencies, obj corev1.ObjectReference, finalizer string) error {
	objObj, err := GetResource[client.Object](ctx, deps.Client, obj.Namespace, obj.Name)
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}

	return AddFinalizer(ctx, deps, objObj, finalizer)
}

func RemoveFinalizer(ctx context.Context, deps *Dependencies, obj client.Object, finalizer string) error {
	if !ContainsFinalizer(obj, finalizer) {
		deps.Logger.Debugw("Finalizer not found", "finalizer", finalizer, "obj", obj.GetName())
		return nil
	}
	controllerutil.RemoveFinalizer(obj, finalizer)
	if err := deps.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	deps.Logger.Debugw("Finalizer removed", "finalizer", finalizer, "obj", obj.GetName())
	return nil
}

func RemoveFinalizerWithRef(ctx context.Context, deps *Dependencies, obj corev1.ObjectReference, finalizer string) error {
	objObj, err := GetResource[client.Object](ctx, deps.Client, obj.Namespace, obj.Name)
	if err != nil {
		return fmt.Errorf("failed to get object: %w", err)
	}
	return RemoveFinalizer(ctx, deps, objObj, finalizer)
}
