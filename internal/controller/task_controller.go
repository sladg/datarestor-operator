package controller

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	reconcile_util "github.com/sladg/datarestor-operator/internal/controller/reconiler"
	task_util "github.com/sladg/datarestor-operator/internal/controller/task"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"

	batchv1 "k8s.io/api/batch/v1"
)

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks/finalizers,verbs=update
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

type TasksReconciler struct {
	Deps *utils.Dependencies
}

func NewTasksReconciler(deps *utils.Dependencies) *TasksReconciler {
	return &TasksReconciler{Deps: deps}
}

func (r *TasksReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Deps.Logger.Named("[Tasks]").With("name", req.Name, "namespace", req.Namespace)
	logger.Info("Tasks reconciliation")

	// Fetch the Task instance
	task := &v1.Task{}
	reconcile, period, err := reconcile_util.CheckResource(ctx, r.Deps, req, task)
	if reconcile {
		return ctrl.Result{RequeueAfter: period}, err
	}

	doObject := func() error { return r.Deps.Update(ctx, task) }
	doStatus := func() error { return r.Deps.Status().Update(ctx, task) }
	isActive := func() bool {
		return task.Status.State == v1.TaskStateRunning || task.Status.State == v1.TaskStatePending
	}

	// Delete task if it is being deleted
	reconcile, period, err = reconcile_util.DeleteResourceWithConditionalFn(ctx, r.Deps, task, constants.TaskFinalizer, isActive)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// ------------ Add finalizer ------------
	reconcile, period, err = reconcile_util.InitResource(ctx, r.Deps, task, constants.TaskFinalizer)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// ------------ Init task ------------
	reconcile, period, err = task_util.Init(ctx, r.Deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// ------------ Scale down ------------
	reconcile, period, err = task_util.CheckTaskScaleDown(ctx, r.Deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// ------------ Start task ------------
	reconcile, period, err = task_util.Start(ctx, r.Deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// ------------ Update task status ------------
	reconcile, period, err = task_util.UpdateTaskStatus(ctx, r.Deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// ------------ Scale up ------------
	reconcile, period, err = task_util.CheckTaskScaleUp(ctx, r.Deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	logger.Info("Tasks reconciliation completed")

	return ctrl.Result{}, nil
}

func (r *TasksReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Task{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
