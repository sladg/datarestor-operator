package reconcile_util

import (
	"context"
	"time"

	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func InitResourceIfConditionMet(ctx context.Context, deps *utils.Dependencies, resource client.Object, finalizer string, conditionFn func() bool) (reconcile bool, period time.Duration, err error) {
	logger := deps.Logger.Named("[InitResourceIfConditionMet]").With("resource", resource.GetName(), "namespace", resource.GetNamespace())

	if !conditionFn() {
		return false, -1, nil
	}

	logger.Infow("Adding finalizer", "finalizer", finalizer)

	updated := controllerutil.AddFinalizer(resource, finalizer)
	if updated {
		return true, constants.ImmediateRequeueInterval, deps.Update(ctx, resource)
	} else {
		return false, -1, nil
	}
}

func CheckResource[T client.Object](ctx context.Context, deps *utils.Dependencies, ref corev1.ObjectReference, resource T) (reconcile bool, period time.Duration, err error) {
	logger := deps.Logger.Named("[CheckResource]").With("resource", ref.Name, "namespace", ref.Namespace)

	isNotFound, isError, err := utils.IsObjectNotFound(ctx, deps, ref, resource)
	if isNotFound {
		logger.Warn("Resource not found, ignoring ...")
		return true, -1, nil
	} else if isError {
		logger.Errorw("Failed to get resource", err)
		return true, constants.DefaultRequeueInterval, err
	}

	return false, constants.ImmediateRequeueInterval, nil
}
