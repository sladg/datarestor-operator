package controller

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	task_util "github.com/sladg/datarestor-operator/internal/controller/task"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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

	task := &v1.Task{}
	isNotFound, isError := utils.IsObjectNotFound(ctx, r.Deps, req, task)
	if isNotFound {
		return ctrl.Result{}, nil
	} else if isError {
		return ctrl.Result{}, fmt.Errorf("failed to get Task")
	}

	now := metav1.Now()

	// If not initialized yet, create job
	if task.Status.InitializedAt.IsZero() {
		task.Status.InitializedAt = &now
		controllerutil.AddFinalizer(task, constants.TaskFinalizer)

		// @TODO: Scale down workloads

		job := batchv1.Job{
			Spec: task.Spec.JobTemplate,
			ObjectMeta: metav1.ObjectMeta{
				Name:      task.Name + "-job",
				Namespace: task.Namespace,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: task.APIVersion,
						Kind:       task.Kind,
						Name:       task.Name,
						UID:        task.UID,
						Controller: ptr.To(true),
					},
				},
			},
		}

		if err := r.Deps.Create(ctx, &job); err != nil {
			logger.Errorw("Failed to create job", err, "job", job.Name)

			task.Status.JobStatus = batchv1.JobStatus{Failed: 1}
			_ = r.Deps.Update(ctx, task)
			return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, err
		} else {
			task.Status.JobStatus = batchv1.JobStatus{Active: 1}
		}

		task.Status.JobRef = corev1.ObjectReference{
			APIVersion: "batch/v1",
			Kind:       "Job",
			Name:       job.Name,
			Namespace:  job.Namespace,
			UID:        job.UID,
		}

		return ctrl.Result{}, r.Deps.Update(ctx, task) // Will be requeued by the job changes
	}

	if task.DeletionTimestamp != nil {
		if task.Status.JobStatus.Active > 0 {
			logger.Info("Task is being deleted, but job is still running, requeuing")
			return ctrl.Result{}, nil // Will be requeued by the job changes
		}

		controllerutil.RemoveFinalizer(task, constants.TaskFinalizer)
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, r.Deps.Update(ctx, task)
	}

	job, err := task_util.GetJob(ctx, r.Deps, task)
	if err != nil {
		logger.Errorw("Failed to get job", err, "job", task.Status.JobRef.Name)
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, err
	}

	task.Status.JobStatus = job.Status

	if job.Status.Active == 0 {
		// @TODO: Bring workloads back up
		controllerutil.RemoveFinalizer(task, constants.TaskFinalizer)
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, r.Deps.Update(ctx, task)
	}

	return ctrl.Result{}, r.Deps.Status().Update(ctx, task)
}

func (r *TasksReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Task{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}
