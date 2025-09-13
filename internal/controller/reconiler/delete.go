package reconcile_util

import (
	"context"
	"time"

	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func CheckDeleteResource(ctx context.Context, deps *utils.Dependencies, resource client.Object, finalizer string) (reconcile bool, period time.Duration, err error) {
	logger := deps.Logger.Named("[CheckDeleteResource]").With(
		"resource", resource.GetName(),
		"namespace", resource.GetNamespace(),
	)

	if resource.GetDeletionTimestamp() == nil {
		return false, -1, nil
	}

	if controllerutil.ContainsFinalizer(resource, finalizer) {
		logger.Info("Finalizer present, continuing ...")
		return false, -1, nil
	}

	return true, constants.ImmediateRequeueInterval, nil
}

// RemoveFinalizerIfConditionMet removes a finalizer when a condition is met, regardless of deletion timestamp
func RemoveFinalizerIfConditionMet(ctx context.Context, deps *utils.Dependencies, resource client.Object, finalizer string, conditionFn func() bool) (reconcile bool, period time.Duration, err error) {
	logger := deps.Logger.Named("[RemoveFinalizerIfConditionMet]").With(
		"resource", resource.GetName(),
		"namespace", resource.GetNamespace(),
	)

	if !conditionFn() {
		return false, -1, nil
	}

	updated := controllerutil.RemoveFinalizer(resource, finalizer)
	if updated {
		logger.Info("Condition met, finalizer removed")
		return true, constants.ImmediateRequeueInterval, deps.Update(ctx, resource)
	} else {
		return false, -1, nil
	}
}
