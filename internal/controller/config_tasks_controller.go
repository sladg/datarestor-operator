package controller

import (
	"context"

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

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs,verbs=get;list;watch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks,verbs=get;list;watch

func (r *ConfigTasksReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Deps.Logger.Named("[ConfigTasksReconciler]").With("name", req.Name, "namespace", req.Namespace)
	logger.Info("Config tasks reconciliation")

	// config := &v1.Config{}
	// isNotFound, isError, _ := utils.IsObjectNotFound(ctx, r.Deps, req, config)
	// if isNotFound {
	// 	return ctrl.Result{}, nil
	// } else if isError {
	// 	return ctrl.Result{}, fmt.Errorf("failed to get Config")
	// }

	// // List all tasks created by this config (across namespaces)
	// tasks := &v1.TaskList{}
	// err := r.Deps.List(ctx, tasks, client.MatchingLabels{
	// 	constants.LabelTaskParentName:      config.Name,
	// 	constants.LabelTaskParentNamespace: config.Namespace,
	// })
	// if err != nil {
	// 	logger.Errorw("Failed to list tasks for config", "error", err)
	// 	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, err
	// }

	// // Update statistics based on task statuses
	// config.Status.Statistics = task_util.GetTasksStatus(tasks)

	// logger.Infow("Updated config statistics", "successful", config.Status.Statistics.SuccessfulBackups, "failed", config.Status.Statistics.FailedBackups, "running", config.Status.Statistics.RunningBackups)

	return ctrl.Result{}, nil // r.Deps.Status().Update(ctx, config)
}

func (r *ConfigTasksReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named("config-tasks").
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Watches(
			&v1.Task{},
			handler.EnqueueRequestsFromMapFunc(watches.RequestTasksForConfig(r.Deps)),
		).
		Complete(r)
}
