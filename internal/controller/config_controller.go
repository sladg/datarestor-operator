package controller

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	config_util "github.com/sladg/datarestor-operator/internal/controller/config"
	reconcile_util "github.com/sladg/datarestor-operator/internal/controller/reconiler"
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

// ------------------------------------------------------------
// ConfigReconciler RBAC permissions
//
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks,verbs=get;list;watch;create;update;patch;delete
//
// ------------------------------------------------------------
// Kubernetes resources the controller interacts with:
//
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="apps",resources=deployments;replicasets;statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch

//nolint:gocyclo
func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Deps.Logger.Named("[ConfigReconciler]").With("name", req.Name, "namespace", req.Namespace)
	logger.Debug("Config reconciliation")

	// Create a new Dependencies struct with the named logger for utility functions
	deps := &utils.Dependencies{
		Client: r.Deps.Client,
		Scheme: r.Deps.Scheme,
		Config: r.Deps.Config,
		Logger: logger,
	}

	// Selectors can be provided on backupconfig-level or on repository-level (repository level takes precedence).
	// AutoRestore and StopPods can be provided on backupconfig-level or on repository-level (repository level takes precedence).
	// If autorestore on backupconfig is true, it will be used for all repositories (unless turned-off by repository level).
	// If stoppods on backupconfig is true, it will be used for all repositories (unless turned-off by repository level).
	// If PVC is deleted, backup/restore job will be deleted. Our reference will throw not found error. We will clear workloadinfo annotation.

	// Fetch the BackupConfig instance
	config := &v1.Config{}

	doObject := func() error { return deps.Update(ctx, config) }
	doStatus := func() error { return deps.Status().Update(ctx, config) }
	doDelete := func() error { return deps.Delete(ctx, config) }

	// Check if config exists, return if not found or error
	reconcile, period, err := reconcile_util.CheckResource(ctx, deps, corev1.ObjectReference{Name: req.Name, Namespace: req.Namespace}, config)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, nil); handled {
		return res, err
	}

	isNew := func() bool {
		return config.Status.InitializedAt.IsZero()
	}

	// --------- Delete resource ---------
	// If deleting, allow it. Remove any tasks references and remove finalizer
	reconcile, period, err = reconcile_util.CheckDeleteResource(ctx, deps, config, constants.ConfigFinalizer)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doDelete); handled {
		return res, err
	}

	// --------- Add finalizer ---------
	reconcile, period, err = reconcile_util.InitResourceIfConditionMet(ctx, deps, config, constants.ConfigFinalizer, isNew)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// --------- Initialize config ---------
	reconcile, period, err = config_util.Init(ctx, deps, config)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// --------- Sync repositories ---------
	reconcile, period, err = config_util.SyncRepositories(ctx, deps, config)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// --------- Initialize repositories ---------
	reconcile, period, err = config_util.InitRepositories(ctx, deps, config)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	pvcResult := config_util.ListPVCsForConfig(ctx, deps, config)

	// @TODO: save information from restic for later use. Peridically re-check
	// --------- Sync repository backup list from restic snapshots (early, on creation) ---------
	// reconcile, period, err = config_util.SyncRepositoryBackups(ctx, r.Deps, config, pvcResult)
	// if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
	// 	return res, err
	// }

	// --------- Auto restore ---------
	reconcile, period, err = config_util.AutoRestore(ctx, deps, config, pvcResult)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, nil); handled {
		return res, err
	}

	// --------- Config restore ---------
	reconcile, period, err = config_util.ConfigRestore(ctx, deps, config, pvcResult)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// ------------ Process force-backup annotation on config ------------
	reconcile, period, _ = config_util.ConfigBackup(ctx, deps, config, pvcResult)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
		return res, err
	}

	// Process PVCs one by one. If one matches, create Task and start reconciliation again.
	for _, pvc := range pvcResult.MatchedPVCs {
		// ------------------  Check for restore annotation ------------------
		reconcile, period, err = config_util.PVCRestore(ctx, deps, config, pvc)
		if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
			return res, err
		}

		// ------------------  Check for backup annotation ------------------
		reconcile, period, err = config_util.PVCBackup(ctx, deps, config, pvc)
		if handled, res, err := reconcile_util.Step(err, reconcile, period, doObject); handled {
			return res, err
		}
	}

	// Process scheduled backups for each repository. Run one-by-one.
	reconcile, period, _ = config_util.ScheduleBackupRepositories(ctx, deps, config, pvcResult)
	if handled, res, err := reconcile_util.Step(err, reconcile, period, doStatus); handled {
		return res, err
	}

	// @TODO: Allow for repositories to be annotated for restore/backup

	// @TODO: Add/Remove finalizer based on tasks running

	logger.Info("Config reconciliation completed, requeueing")

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
