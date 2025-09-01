package controller

import (
	"context"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	"github.com/sladg/autorestore-backup-operator/internal/controller/watches"

	logic "github.com/sladg/autorestore-backup-operator/internal/logic/backupconfig"
)

// NewBackupConfigReconciler creates a new BackupConfigReconciler
func NewBackupConfigReconciler(deps *utils.Dependencies) (*BackupConfigReconciler, error) {
	return &BackupConfigReconciler{
		Deps:       deps,
		cronParser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}, nil
}

// BackupConfigReconciler reconciles a BackupConfig object
type BackupConfigReconciler struct {
	Deps       *utils.Dependencies
	cronParser cron.Parser
}

// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=backupconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=backupconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=backupconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments;statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the BackupConfig object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *BackupConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Deps.Logger.With("name", req.Name, "namespace", req.Namespace)

	// Fetch the BackupConfig instance
	backupConfig := &backupv1alpha1.BackupConfig{}
	if err := r.Deps.Client.Get(ctx, req.NamespacedName, backupConfig); err != nil {
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

	// Discover and match PVCs
	managedPVCs, err := logic.DiscoverMatchingPVCs(ctx, r.Deps, backupConfig)
	if err != nil {
		log.Errorw("Failed to discover and match PVCs", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Update status with managed PVCs count
	if err := logic.UpdateStatusPVCsCount(ctx, r.Deps, backupConfig, managedPVCs); err != nil {
		log.Errorw("Failed to update managed PVCs status", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Handle auto-restore logic
	if err := logic.HandleAutoRestore(ctx, r.Deps, backupConfig, managedPVCs); err != nil {
		log.Errorw("Failed to handle auto-restore", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Handle manual backup/restore annotations
	if err := logic.HandleAnnotations(ctx, r.Deps, backupConfig, managedPVCs); err != nil {
		log.Errorw("Failed to handle annotations", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Handle scheduled backups on a per-target basis
	if err := logic.HandleScheduledBackups(ctx, r.Deps, backupConfig, managedPVCs, &r.cronParser); err != nil {
		log.Errorw("Failed to handle scheduled backups", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// 9. TODO: Handle retention policy on a per-target basis

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.BackupConfig{}).
		Owns(&backupv1alpha1.ResticRepository{}).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(watches.FindObjectsForPVC(mgr.GetClient())),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(watches.FindObjectsForPod(mgr.GetClient())),
		).
		Complete(r)
}
