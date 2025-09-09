package utils

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
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

func IsObjectNotFound(ctx context.Context, deps *Dependencies, req ctrl.Request, obj client.Object) (bool, bool) {
	log := deps.Logger.Named("[IsObjectNotFound]")

	err := deps.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, obj)
	isNotFound := client.IgnoreNotFound(err) != nil
	if err != nil {
		log.Errorw("Failed to get object", "error", err, "name", req.Name, "namespace", req.Namespace)

		return false, true
	} else if isNotFound {
		log.Info("Object not found - must be deleted, ignoring", "name", req.Name, "namespace", req.Namespace)

		return true, false
	}

	return false, false
}
