package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/logic/resticbackup"
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

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch

// Reconcile handles ResticBackup lifecycle
func (r *ResticBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Create component-specific logger for inheritance
	logger := r.Deps.Logger.Named("[ResticBackup]").With("name", req.Name, "namespace", req.Namespace)

	// Create dependencies with the named logger for inheritance
	deps := *r.Deps
	deps.Logger = logger

	resticBackup := &v1.ResticBackup{}
	if err := r.Deps.Get(ctx, req.NamespacedName, resticBackup); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("ResticBackup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Errorw("Failed to get ResticBackup", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if resticBackup.DeletionTimestamp != nil {
		resticBackup.Status.Phase = v1.PhaseDeletion
		if err := r.Deps.Status().Update(ctx, resticBackup); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
	}

	// Handle different phases with dynamic phase checking
	switch resticBackup.Status.Phase {
	case v1.PhaseUnknown:
		return resticbackup.HandleBackupUnknown(ctx, &deps, resticBackup)
	case v1.PhasePending:
		return resticbackup.HandleBackupPending(ctx, &deps, resticBackup)
	case v1.PhaseRunning:
		return resticbackup.HandleBackupRunning(ctx, &deps, resticBackup)
	case v1.PhaseCompleted:
		return resticbackup.HandleBackupCompleted(ctx, &deps, resticBackup)
	case v1.PhaseFailed:
		return resticbackup.HandleBackupFailed(ctx, &deps, resticBackup)
	case v1.PhaseDeletion:
		return resticbackup.HandleBackupDeletion(ctx, &deps, resticBackup)
	default:
		logger.Infow("Unknown or unhandled phase", "phase", resticBackup.Status.Phase)
		return ctrl.Result{}, fmt.Errorf("unknown or unhandled phase: %s", resticBackup.Status.Phase)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResticBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ResticBackup{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
