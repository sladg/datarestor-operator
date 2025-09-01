package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	logic "github.com/sladg/autorestore-backup-operator/internal/logic/resticbackup"
)

// ResticBackupReconciler reconciles a ResticBackup object
type ResticBackupReconciler struct {
	Deps *utils.Dependencies
}

// NewResticBackupReconciler creates a new ResticBackupReconciler
func NewResticBackupReconciler(deps *utils.Dependencies) *ResticBackupReconciler {
	return &ResticBackupReconciler{
		Deps: deps,
	}
}

// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch

// Reconcile handles ResticBackup lifecycle
func (r *ResticBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Deps.Logger.With("name", req.Name, "namespace", req.Namespace)

	backup := &v1.ResticBackup{}
	if err := r.Deps.Client.Get(ctx, req.NamespacedName, backup); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ResticBackup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Errorw("Failed to get ResticBackup", "error", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if backup.DeletionTimestamp != nil {
		return logic.HandleBackupDeletion(ctx, r.Deps, backup)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(backup, constants.ResticBackupFinalizer) {
		controllerutil.AddFinalizer(backup, constants.ResticBackupFinalizer)
		if err := r.Deps.Client.Update(ctx, backup); err != nil {
			log.Errorw("add finalizer failed", "error", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Use a pipeline to handle different phases
	return utils.ProcessSteps(
		utils.Step{
			Condition: func() bool { return backup.Status.Phase == "" || backup.Status.Phase == v1.PhasePending },
			Action:    func() (ctrl.Result, error) { return logic.HandleBackupPending(ctx, r.Deps, backup, false) },
		},
		utils.Step{
			Condition: func() bool { return backup.Status.Phase == v1.PhaseRunning },
			Action:    func() (ctrl.Result, error) { return logic.HandleBackupRunning(ctx, r.Deps, backup) },
		},
		utils.Step{
			Condition: func() bool { return backup.Status.Phase == v1.PhaseCompleted },
			Action:    func() (ctrl.Result, error) { return logic.HandleBackupCompleted(ctx, r.Deps, backup) },
		},
		utils.Step{
			Condition: func() bool { return backup.Status.Phase == v1.PhaseFailed },
			Action:    func() (ctrl.Result, error) { return logic.HandleBackupFailed(ctx, r.Deps, backup) },
		},
		utils.Step{
			Condition: func() bool { return true }, // Default case
			Action: func() (ctrl.Result, error) {
				r.Deps.Logger.Infow("Unknown or unhandled phase", "phase", backup.Status.Phase)
				return logic.HandlePendingPhase(ctx, r.Deps, backup)
			},
		},
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResticBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ResticBackup{}).
		Complete(r)
}
