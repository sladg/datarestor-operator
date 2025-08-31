package controller

import (
	"context"

	"github.com/robfig/cron/v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	"github.com/sladg/autorestore-backup-operator/internal/stubs"
)

// NewBackupConfigReconciler creates a new BackupConfigReconciler
func NewBackupConfigReconciler(client client.Client, scheme *runtime.Scheme, config *rest.Config) (*BackupConfigReconciler, error) {
	return &BackupConfigReconciler{
		Client:     client,
		Scheme:     scheme,
		Config:     config,
		cronParser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}, nil
}

// BackupConfigReconciler reconciles a BackupConfig object
type BackupConfigReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Config     *rest.Config
	cronParser cron.Parser
}

// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=backupconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=backupconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=backupconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrepositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=persistentvolumeclaims/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core,resources=events,verbs=create;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *BackupConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := utils.LoggerFrom(ctx, "backupconfig-reconciler").WithValues("name", req.Name, "namespace", req.Namespace)
	// Minimal scheduling retained; heavy logic trimmed
	backupConfig := &backupv1alpha1.BackupConfig{}
	err := r.Get(ctx, req.NamespacedName, backupConfig)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request with backoff.
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// Check if BackupConfig is being deleted
	if !backupConfig.DeletionTimestamp.IsZero() {
		return stubs.HandleBackupConfigDeletion(ctx, r.Client, backupConfig)
	}

	// Add finalizer if not present (highest priority)
	if !controllerutil.ContainsFinalizer(backupConfig, constants.BackupConfigFinalizer) {
		return logger.WithLogging(ctx, "addFinalizer", func(ctx context.Context) (ctrl.Result, error) {
			backupConfig.Finalizers = append(backupConfig.Finalizers, constants.BackupConfigFinalizer)
			if err := r.Update(ctx, backupConfig); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		})
	}

	// Ensure ResticRepository CRDs exist for all backup targets
	if err := stubs.EnsureResticRepositories(ctx, r.Client, r.Scheme, backupConfig); err != nil {
		logger.Failed("ensure restic repositories", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Update job statistics (non-critical, can fail)
	if err := stubs.UpdateJobStatistics(ctx, r.Client, backupConfig); err != nil {
		logger.WithValues("error", err).Debug("Failed to update job statistics")
	}

	// Apply retention policy daily (non-critical, can fail)
	if stubs.ShouldApplyRetentionPolicy(backupConfig, r.cronParser) {
		logger.WithLogging(ctx, "applyRetentionPolicy", func(ctx context.Context) (ctrl.Result, error) {
			if err := stubs.ApplyRetentionPolicy(ctx, r.Client, backupConfig); err != nil {
				return ctrl.Result{}, err
			}
			// Update last retention check time
			now := metav1.Now()
			backupConfig.Status.LastRetentionCheck = &now
			if err := r.Status().Update(ctx, backupConfig); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{}, nil
		})
	}

	// Find PVCs that match the selector
	matchedPVCs, err := stubs.FindMatchingPVCs(ctx, r.Client, backupConfig)
	if err != nil {
		logger.Failed("find matching PVCs", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Update status with managed PVCs
	if err := stubs.UpdateManagedPVCsStatus(ctx, r.Client, backupConfig, matchedPVCs); err != nil {
		logger.Failed("update managed PVCs status", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// Handle job scheduling based on matched PVCs
	if result, err := stubs.HandleJobSchedulingOriginal(ctx, r.Client, r.Scheme, backupConfig, matchedPVCs, r.cronParser); err != nil {
		logger.Failed("handle job scheduling", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	} else if !result.IsZero() {
		// Early return requested by scheduling logic
		return result, nil
	}

	// Schedule next reconciliation with a default interval
	return ctrl.Result{RequeueAfter: constants.DefaultReconcileInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *BackupConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create field index for finding pods by PVC claim name
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, "spec.volumes.persistentVolumeClaim.claimName", stubs.IndexPodByPVCClaimName); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.BackupConfig{}).
		Watches(
			&corev1.PersistentVolumeClaim{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForPVC),
		).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForPod),
		).
		Complete(r)
}

func (r *BackupConfigReconciler) findObjectsForPVC(ctx context.Context, obj client.Object) []reconcile.Request {
	return stubs.FindObjectsForPVC(ctx, r.Client, obj)
}

func (r *BackupConfigReconciler) findObjectsForPod(ctx context.Context, obj client.Object) []reconcile.Request {
	return stubs.FindObjectsForPod(ctx, r.Client, obj)
}
