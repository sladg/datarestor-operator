package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"

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
	log := r.Deps.Logger.With("name", req.Name, "namespace", req.Namespace)

	resticBackup := &v1.ResticBackup{}
	if err := r.Deps.Get(ctx, req.NamespacedName, resticBackup); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ResticBackup resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Errorw("Failed to get ResticBackup", "error", err)
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

	p := resticBackup.Status.Phase

	return utils.ProcessSteps(
		utils.Step{
			Condition: func() bool { return p == v1.PhaseUnknown || p == v1.PhasePending },
			Action:    func() (ctrl.Result, error) { return resticbackup.HandleBackupPending(ctx, r.Deps, resticBackup) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseRunning },
			Action:    func() (ctrl.Result, error) { return resticbackup.HandleBackupRunning(ctx, r.Deps, resticBackup) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseCompleted },
			Action:    func() (ctrl.Result, error) { return resticbackup.HandleBackupCompleted(ctx, r.Deps, resticBackup) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseFailed },
			Action:    func() (ctrl.Result, error) { return resticbackup.HandleBackupFailed(ctx, r.Deps, resticBackup) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseDeletion },
			Action:    func() (ctrl.Result, error) { return resticbackup.HandleBackupDeletion(ctx, r.Deps, resticBackup) },
		},
		utils.Step{
			Condition: func() bool { return true }, // Default case
			Action: func() (ctrl.Result, error) {
				r.Deps.Logger.Infow("Unknown or unhandled phase", "phase", p)
				return ctrl.Result{}, fmt.Errorf("unknown or unhandled phase: %s", p)
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
