package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
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
// +kubebuilder:rbac:groups="apps",resources=deployments;statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *BackupConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Deps.Logger.With("name", req.Name, "namespace", req.Namespace)

	// Fetch the BackupConfig instance
	backupConfig := &v1.BackupConfig{}
	if err := r.Deps.Get(ctx, req.NamespacedName, backupConfig); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("BackupConfig resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Errorw("Failed to get BackupConfig", "error", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if backupConfig.DeletionTimestamp != nil {
		return logic.HandleBackupConfigDeletion(ctx, r.Deps, backupConfig)
	}

	// Add finalizer if not present
	if err := utils.AddFinalizer(ctx, r.Deps, backupConfig, constants.BackupConfigFinalizer); err != nil {
		log.Errorw("Failed to add finalizer", "error", err)
		return ctrl.Result{}, err
	}

	// Ensure ResticRepository CRDs exist for all backup targets
	if err := logic.EnsureResticRepositories(ctx, r.Deps, backupConfig); err != nil {
		log.Errorw("Failed to ensure restic repositories", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Discover and match PVCs using BackupConfig selectors
	pvcList := &corev1.PersistentVolumeClaimList{}
	pvcs, err := utils.FindMatchingResources[*corev1.PersistentVolumeClaim](ctx, r.Deps, backupConfig.Spec.Selectors, pvcList)
	if err != nil {
		log.Errorw("Failed to find managed PVCs", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Handle auto-restore logic
	if err := logic.HandleAutoRestore(ctx, r.Deps, backupConfig, pvcs); err != nil {
		log.Errorw("Failed to handle auto-restore", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Handle manual backup/restore annotations
	if err := logic.HandleAnnotations(ctx, r.Deps, backupConfig, pvcs); err != nil {
		log.Errorw("Failed to handle annotations", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Handle scheduled backups on a per-target basis
	// if err := logic.HandleScheduledBackups(ctx, r.Deps, backupConfig, pvcs); err != nil {
	// 	log.Errorw("Failed to handle scheduled backups", "error", err)
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
