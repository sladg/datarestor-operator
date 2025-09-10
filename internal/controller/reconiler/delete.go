package reconcile_util

import (
	"context"
	"time"

	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func DeleteResourceWithFinalizer(ctx context.Context, deps *utils.Dependencies, resource client.Object, finalizer string, updateFunc func() error) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[DeleteResourceWithFinalizer]").With(
		"resource", resource.GetName(),
		"namespace", resource.GetNamespace(),
	)

	if resource.GetDeletionTimestamp() == nil {
		return false, -1, nil
	}

	if !controllerutil.ContainsFinalizer(resource, finalizer) {
		return false, -1, nil
	}

	logger.Infow("Removing finalizer and allowing deletion", "deletionTimestamp", resource.GetDeletionTimestamp())

	controllerutil.RemoveFinalizer(resource, finalizer)

	if err := updateFunc(); err != nil {
		return false, constants.ImmediateRequeueInterval, err
	}

	return true, constants.ImmediateRequeueInterval, deps.Update(ctx, resource)
}

func DeleteResourceWithConditionalFn(ctx context.Context, deps *utils.Dependencies, resource client.Object, finalizer string, conditionFn func() bool) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[DeleteResourceWithConditionalFn]").With(
		"resource", resource.GetName(),
		"namespace", resource.GetNamespace(),
	)

	if resource.GetDeletionTimestamp() == nil {
		return false, -1, nil
	}

	if !controllerutil.ContainsFinalizer(resource, finalizer) {
		return false, -1, nil
	}

	if !conditionFn() {
		logger.Infow("Condition not met, requeuing", "deletionTimestamp", resource.GetDeletionTimestamp())
		return false, constants.DefaultRequeueInterval, nil
	}

	controllerutil.RemoveFinalizer(resource, finalizer)

	return true, constants.ImmediateRequeueInterval, nil
}
