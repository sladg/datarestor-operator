package controller

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	reconcile_util "github.com/sladg/datarestor-operator/internal/controller/reconiler"
	task_util "github.com/sladg/datarestor-operator/internal/controller/task"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
	logger := r.Deps.Logger.Named("[TasksReconciler]").With("name", req.Name, "namespace", req.Namespace)
	logger.Debug("Tasks reconciliation")

	// Create a new Dependencies struct with the named logger for utility functions
	deps := &utils.Dependencies{
		Client: r.Deps.Client,
		Scheme: r.Deps.Scheme,
		Config: r.Deps.Config,
		Logger: logger,
	}

	// Fetch the Task instance
	task := &v1.Task{}

	doObject := func() error { return deps.Update(ctx, task) }
	doStatus := func() error { return deps.Status().Update(ctx, task) }
	doDelete := func() error { return deps.Delete(ctx, task) }

	// Check if task exists, delete if not
	reconcile, period, err := reconcile_util.CheckResource(ctx, deps, corev1.ObjectReference{Name: req.Name, Namespace: req.Namespace}, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doDelete); handled {
		return res, err
	}

	// Check if task is in a final state (completed or failed)
	isFinished := func() bool {
		return task.Status.State == v1.TaskStateCompleted || task.Status.State == v1.TaskStateFailed
	}

	isNew := func() bool {
		return task.Status.InitializedAt.IsZero()
	}

	// If task is completed/failed, remove finalizer
	reconcile, period, err = reconcile_util.RemoveFinalizerIfConditionMet(ctx, deps, task, constants.TaskFinalizer, isFinished)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// Delete as needed - finalizer will block deletion
	reconcile, period, err = reconcile_util.CheckDeleteResource(ctx, deps, task, constants.TaskFinalizer)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doDelete); handled {
		return res, err
	}

	// ------------ Add finalizer ------------
	reconcile, period, err = reconcile_util.InitResourceIfConditionMet(ctx, deps, task, constants.TaskFinalizer, isNew)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// ------------ State progression: Pending -> ScalingDown -> Starting -> Running -> ScalingUp -> Completed ------------

	// ------------ Init task ------------
	// Init task (Pending -> ScalingDown)
	reconcile, period, err = task_util.Init(ctx, deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// ------------ Handle scale down ------------
	// Handle scale down (Pending -> ScalingDown -> Starting)
	reconcile, period, err = task_util.CheckTaskScaleDown(ctx, deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// ------------ Handle job start ------------
	// Handle job start (Starting -> Running)
	reconcile, period, err = task_util.Start(ctx, deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// ------------ Handle job status update ------------
	// Handle job status update (Running -> ScalingUp/Completed)
	reconcile, period, err = task_util.UpdateTaskStatus(ctx, deps, task)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// ------------ Handle scale up ------------
	// Handle scale up (ScalingUp -> Completed)
	reconcile, period, err = task_util.CheckTaskScaleUp(ctx, deps, task)
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
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
