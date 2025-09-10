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

func IsObjectNotFound(ctx context.Context, deps *Dependencies, req ctrl.Request, obj client.Object) (bool, bool, error) {
	log := deps.Logger.Named("[IsObjectNotFound]").With("name", req.Name, "namespace", req.Namespace)

	err := deps.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, obj)

	isNotFound := client.IgnoreNotFound(err) != nil
	if isNotFound {
		log.Infow("Object not found - must be deleted, ignoring")
		return true, false, nil
	} else if err != nil {
		log.Warnw("Failed to get object")
		return false, true, err
	}

	return false, false, nil
}
