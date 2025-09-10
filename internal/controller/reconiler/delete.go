package reconcile_util

import (
	"context"
	"time"

	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func DeleteResource(ctx context.Context, deps *utils.Dependencies, resource client.Object, finalizer string) (bool, time.Duration, error) {
	controllerutil.RemoveFinalizer(resource, finalizer)

	// return time.Duration(0), nil

	return true, constants.ImmediateRequeueInterval, nil
}

func DeleteResourceWithFinalizer(ctx context.Context, deps *utils.Dependencies, resource client.Object, finalizer string, updateFunc func(ctx context.Context, obj client.Object) error) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[DeleteResourceWithFinalizer]").With("resource", resource.GetName(), "namespace", resource.GetNamespace())

	if resource.GetDeletionTimestamp() == nil {
		return false, 0, nil
	}

	if !controllerutil.ContainsFinalizer(resource, finalizer) {
		return false, 0, nil
	}

	logger.Infow("Something is being deleted", "resource", resource.GetName(), "deletionTimestamp", resource.GetDeletionTimestamp())

	controllerutil.RemoveFinalizer(resource, finalizer)

	if err := updateFunc(ctx, resource); err != nil {
		return false, constants.ImmediateRequeueInterval, err
	}

	return true, constants.ImmediateRequeueInterval, deps.Update(ctx, resource)
}
