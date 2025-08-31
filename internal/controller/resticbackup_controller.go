package controller

import (
	"context"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	"github.com/sladg/autorestore-backup-operator/internal/stubs"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ResticBackupReconciler reconciles a ResticBackup object
type ResticBackupReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

// NewResticBackupReconciler creates a new ResticBackupReconciler
func NewResticBackupReconciler(client client.Client, scheme *runtime.Scheme, config *rest.Config) *ResticBackupReconciler {
	return &ResticBackupReconciler{
		Client: client,
		Scheme: scheme,
		Config: config,
	}
}

// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrepositories,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles ResticBackup lifecycle
func (r *ResticBackupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := utils.LoggerFrom(ctx, "restic-backup").
		WithValues("name", req.Name, "namespace", req.Namespace)
	logger.Starting("reconcile")

	// Fetch the ResticBackup instance
	backup := &backupv1alpha1.ResticBackup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Failed("get backup", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if backup.DeletionTimestamp != nil {
		return stubs.HandleBackupDeletion(ctx, r.Client, r.Scheme, backup)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(backup, constants.ResticBackupFinalizer) {
		controllerutil.AddFinalizer(backup, constants.ResticBackupFinalizer)
		if err := r.Update(ctx, backup); err != nil {
			logger.Failed("add finalizer", err)
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle backup lifecycle based on current phase
	return utils.ProcessSteps(
		utils.Step{
			Condition: func() bool { return backup.Status.Phase == "" || backup.Status.Phase == "Pending" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleBackupPending(ctx, r.Client, r.Scheme, backup)
			},
		},
		utils.Step{
			Condition: func() bool { return backup.Status.Phase == "Running" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleBackupRunning(ctx, r.Client, r.Scheme, backup)
			},
		},
		utils.Step{
			Condition: func() bool { return backup.Status.Phase == "Ready" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleBackupReady(ctx, r.Client, r.Scheme, backup)
			},
		},
		utils.Step{
			Condition: func() bool { return backup.Status.Phase == "Failed" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleBackupFailed(ctx, r.Client, r.Scheme, backup)
			},
		},
		utils.Step{
			Condition: func() bool { return true }, // Default case
			Action: func() (ctrl.Result, error) {
				utils.LoggerFrom(ctx, "restic-backup").
					WithValues("name", req.Name, "namespace", req.Namespace).
					WithValues("phase", backup.Status.Phase).Debug("Unknown phase")
				return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
			},
		},
	)
}

// SetupWithManager sets up the controller with the Manager
func (r *ResticBackupReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.ResticBackup{}).
		Complete(r)
}
