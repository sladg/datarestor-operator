package controller

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/controller/watches"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type ConfigTasksReconciler struct {
	Deps *utils.Dependencies
}

func NewConfigTasksReconciler(deps *utils.Dependencies) *ConfigTasksReconciler {
	return &ConfigTasksReconciler{Deps: deps}
}

func (r *ConfigTasksReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// logger := r.Deps.Logger.Named("[ConfigTasksReconciler]").With("name", req.Name, "namespace", req.Namespace)

	config := &v1.Config{}
	isNotFound, isError := utils.IsObjectNotFound(ctx, r.Deps, req, config)
	if isNotFound {
		return ctrl.Result{}, nil
	} else if isError {
		return ctrl.Result{}, fmt.Errorf("failed to get Config: %w", isError)
	}

	// tasks := &v1.TaskList{}
	// err := r.Deps.List(ctx, tasks, client.MatchingLabels{
	// 	constants.LabelTaskParentName:      config.Name,
	// 	constants.LabelTaskParentNamespace: config.Namespace,
	// })
	// if err != nil {
	// 	logger.Errorw("Failed to list tasks for config", "error", err)
	// 	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, err
	// }
	// config.Status.Tasks = task_util.GetTasksStatus(tasks)

	return ctrl.Result{}, r.Deps.Status().Update(ctx, config)
}

func (r *ConfigTasksReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Config{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Watches(
			&v1.Task{},
			handler.EnqueueRequestsFromMapFunc(watches.RequestTasksForConfig(r.Deps)),
		).
		Complete(r)
}
