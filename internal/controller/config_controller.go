package controller

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	config_util "github.com/sladg/datarestor-operator/internal/controller/config"
	reconcile_util "github.com/sladg/datarestor-operator/internal/controller/reconiler"
	task_util "github.com/sladg/datarestor-operator/internal/controller/task"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/controller/watches"

	corev1 "k8s.io/api/core/v1"
)

func NewConfigReconciler(deps *utils.Dependencies) *ConfigReconciler {
	return &ConfigReconciler{Deps: deps}
}

type ConfigReconciler struct {
	Deps *utils.Dependencies
}

// ConfigReconciler RBAC permissions
// Core resources the controller manages:
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs/finalizers,verbs=update
//
// Tasks the controller creates:
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks,verbs=get;list;watch;create
//
// Kubernetes resources the controller interacts with:
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",resources=deployments;replicasets;statefulsets,verbs=get;list;watch;update;patch

//nolint:gocyclo
func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Deps.Logger.Named("[ConfigReconciler]").With("name", req.Name, "namespace", req.Namespace)
	logger.Info("Config reconciliation")

	// Selectors can be provided on backupconfig-level or on repository-level (repository level takes precedence).
	// AutoRestore and StopPods can be provided on backupconfig-level or on repository-level (repository level takes precedence).
	// If autorestore on backupconfig is true, it will be used for all repositories (unless turned-off by repository level).
	// If stoppods on backupconfig is true, it will be used for all repositories (unless turned-off by repository level).
	// If PVC is deleted, backup/restore job will be deleted. Our reference will throw not found error. We will clear workloadinfo annotation.

	// Fetch the BackupConfig instance
	config := &v1.Config{}

	// Check if config exists, return if not found or error
	reconcile, period, err := reconcile_util.CheckResource(ctx, r.Deps, req, config)
	if reconcile {
		return ctrl.Result{RequeueAfter: period}, err
	}

	doStatus := func() error { return r.Deps.Status().Update(ctx, config) }
	doObject := func() error { return r.Deps.Update(ctx, config) }
	noUpdate := func() error { return nil }

	// --------- Delete resource ---------
	// If deleting, allow it. Remove any tasks references and remove finalizer
	reconcile, period, err = reconcile_util.DeleteResourceWithFinalizer(ctx, r.Deps, config, constants.ConfigFinalizer, func(ctx context.Context, obj client.Object) error {
		return task_util.RemoveTasksByConfig(ctx, r.Deps, config)
	})
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// --------- Initialize resource ---------
	// Check if config is initialized, if not, add finalizer and initialize repositories
	reconcile, period, err = reconcile_util.InitResource(ctx, r.Deps, config, constants.ConfigFinalizer)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// --------- Initialize config ---------
	reconcile, period, err = config_util.Init(ctx, r.Deps, config)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// --------- Sync repositories ---------
	reconcile, period, err = config_util.SyncRepositories(ctx, r.Deps, config)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// --------- Initialize repositories ---------
	reconcile, period, err = config_util.InitRepositories(ctx, r.Deps, config)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	pvcResult := config_util.ListPVCsForConfig(ctx, r.Deps, config)

	// --------- Auto restore ---------
	reconcile, period, err = config_util.AutoRestore(ctx, r.Deps, config, pvcResult)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, noUpdate); handled {
		return res, err
	}

	// --------- Config restore ---------
	reconcile, period, err = config_util.ConfigRestore(ctx, r.Deps, config, pvcResult)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// ------------ Process force-backup annotation on config ------------
	reconcile, period, _ = config_util.ConfigBackup(ctx, r.Deps, config, pvcResult)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// Process PVCs one by one. If one matches, create Task and start reconciliation again.
	for _, pvc := range pvcResult.MatchedPVCs {
		// ------------------  Check for restore annotation ------------------
		reconcile, period, err = config_util.PVCRestore(ctx, r.Deps, config, pvc)
		if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
			return res, err
		}

		// ------------------  Check for backup annotation ------------------
		reconcile, period, err = config_util.PVCBackup(ctx, r.Deps, config, pvc)
		if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
			return res, err
		}
	}

	// Process scheduled backups for each repository. Run one-by-one.
	reconcile, period, _ = config_util.ScheduleBackupRepositories(ctx, r.Deps, config, pvcResult)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// @TODO: Allow for repositories to be annotated for restore/backup

	// We need to requeue to allow for scheduled backups to be processed
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Config{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Watches(
			&v1.Config{},
			handler.EnqueueRequestsFromMapFunc(watches.RequestConfigs(r.Deps)),
		).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(watches.RequestPVCsForConfig(r.Deps)),
		).
		Complete(r)
}
