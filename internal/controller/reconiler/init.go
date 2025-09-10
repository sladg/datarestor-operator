package reconcile_util

import (
	"context"
	"time"

	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func InitResource(ctx context.Context, deps *utils.Dependencies, resource client.Object, finalizer string) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[InitResource]").With("resource", resource.GetName(), "namespace", resource.GetNamespace())

	if controllerutil.ContainsFinalizer(resource, finalizer) {
		return false, -1, nil
	}

	if resource.GetDeletionTimestamp() != nil {
		return false, -1, nil
	}

	logger.Info("Adding finalizer", "finalizer", finalizer)

	controllerutil.AddFinalizer(resource, finalizer)

	return true, constants.ImmediateRequeueInterval, nil
}

func CheckResource[T client.Object](ctx context.Context, deps *utils.Dependencies, req ctrl.Request, resource T) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[CheckResource]").With("resource", req.Name, "namespace", req.Namespace)

	isNotFound, isError, err := utils.IsObjectNotFound(ctx, deps, req, resource)
	if isNotFound {
		logger.Warn("Resource not found, might have been deleted")
		return true, 0, nil
	} else if isError {
		logger.Errorw("Failed to get resource", err)
		return true, 0, err
	}

	return false, constants.ImmediateRequeueInterval, nil
}
