package utils

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Contains checks if an element is present in a slice.
func Contains[T comparable](slice []T, element T) bool {
	for _, item := range slice {
		if item == element {
			return true
		}
	}
	return false
}

func IsObjectNotFound(ctx context.Context, deps *Dependencies, req corev1.ObjectReference, obj client.Object) (isNotFound bool, isError bool, err error) {
	log := deps.Logger.Named("[IsObjectNotFound]").With("name", req.Name, "namespace", req.Namespace)

	err = deps.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, obj)

	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			// This is a "not found" error
			log.Infow("Object not found - must be deleted, ignoring")
			return true, false, nil
		} else {
			// This is some other error
			log.Warnw("Failed to get object", "error", err)
			return false, true, err
		}
	}

	// No error means object was found successfully
	return false, false, nil
}
