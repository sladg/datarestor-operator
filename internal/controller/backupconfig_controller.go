package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/controller/watches"

	logic "github.com/sladg/datarestor-operator/internal/logic/backupconfig"
)

func NewBackupConfigReconciler(deps *utils.Dependencies) (*BackupConfigReconciler, error) {
	return &BackupConfigReconciler{
		Deps: deps,
	}, nil
}

type BackupConfigReconciler struct {
	Deps *utils.Dependencies
}

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=backupconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=backupconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=backupconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments;replicasets;statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *BackupConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Deps.Logger.Named("[BackupConfig]").With("name", req.Name, "namespace", req.Namespace)

	// Create dependencies with the named logger for inheritance
	deps := *r.Deps
	deps.Logger = logger

	// Fetch the BackupConfig instance
	backupConfig := &v1.BackupConfig{}
	isNotFound, isError := utils.IsObjectNotFound(ctx, r.Deps, backupConfig)
	if isNotFound {
		return ctrl.Result{}, nil
	} else if isError {
		return ctrl.Result{}, fmt.Errorf("failed to get BackupConfig")
	}

	// Handle deletion
	if backupConfig.DeletionTimestamp != nil {
		// If in active state, allow the rest of code to continue processing
		if !utils.Contains(constants.ActivePhases, backupConfig.Status.Phase) {
			return logic.HandleBackupConfigDeletion(ctx, &deps, backupConfig)
		}
	}

	// Add finalizer if not present
	if err := utils.SetOwnFinalizer(ctx, &deps, backupConfig, constants.BackupConfigFinalizer); err != nil {
		logger.Errorw("Failed to add finalizer", err)
		return ctrl.Result{}, err
	}

	// Ensure ResticRepository CRDs exist for all backup targets
	if err := logic.EnsureResticRepositories(ctx, &deps, backupConfig); err != nil {
		logger.Errorw("Failed to ensure restic repositories", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Discover and match PVCs using BackupConfig selectors
	pvcList := &corev1.PersistentVolumeClaimList{}
	pvcs, err := utils.FindMatchingResources[*corev1.PersistentVolumeClaim](ctx, &deps, backupConfig.Spec.Selectors, pvcList)
	if err != nil {
		logger.Errorw("Failed to find managed PVCs", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Update BackupConfig status with repository information
	if err := logic.UpdateBackupConfigStatus(ctx, &deps, backupConfig, pvcs); err != nil {
		logger.Errorw("Failed to update BackupConfig status", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Handle auto-restore logic
	// if err := logic.HandleAutoRestore(ctx, &deps, backupConfig, pvcs); err != nil {
	// 	logger.Errorw("Failed to handle auto-restore", err)
	// 	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	// }

	// Handle manual backup/restore annotations
	if reconcile, res, err := logic.HandleAnnotations(ctx, &deps, backupConfig, pvcs); err != nil {
		logger.Errorw("Failed to handle annotations", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	} else if reconcile {
		return res, err
	}

	// Handle scheduled backups on a per-target basis
	// if err := logic.HandleScheduledBackups(ctx, &deps, backupConfig, pvcs); err != nil {
	// 	logger.Errorw("Failed to handle scheduled backups",  err)
	// 	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	// }

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

func (r *BackupConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.BackupConfig{}).
		Owns(&v1.ResticRepository{}).
		Owns(&v1.ResticBackup{}).
		Owns(&v1.ResticRestore{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(watches.FindObjectsForPVC(r.Deps)),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(watches.FindObjectsForPod(r.Deps)),
		).
		Complete(r)
}
