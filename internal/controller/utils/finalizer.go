package utils

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ContainsFinalizer checks if a finalizer is present on the object
func ContainsFinalizer(obj client.Object, finalizer string) bool {
	return controllerutil.ContainsFinalizer(obj, finalizer)
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

// RemoveFinalizer removes a finalizer from a resource if it exists
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
