package controller

import (
	"context"
	"fmt"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	task_util "github.com/sladg/datarestor-operator/internal/controller/task"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/controller/watches"
	"github.com/sladg/datarestor-operator/internal/restic"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func NewConfigReconciler(deps *utils.Dependencies) *ConfigReconciler {
	return &ConfigReconciler{Deps: deps}
}

type ConfigReconciler struct {
	Deps *utils.Dependencies
}

// Maybe following: resticbackup's

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;update;patch

// ----

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=configs/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=tasks/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",resources=deployments;replicasets;statefulsets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

//nolint:gocyclo
func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Deps.Logger.Named("[ConfigReconciler]").With("name", req.Name, "namespace", req.Namespace)
	r.Deps.Logger = logger

	// Selectors can be provided on backupconfig-level or on repository-level (repository level takes precedence).
	// AutoRestore and StopPods can be provided on backupconfig-level or on repository-level (repository level takes precedence).
	// If autorestore on backupconfig is true, it will be used for all repositories (unless turned-off by repository level).
	// If stoppods on backupconfig is true, it will be used for all repositories (unless turned-off by repository level).
	// If PVC is deleted, backup/restore job will be deleted. Our reference will throw not found error. We will clear workloadinfo annotation.

	// Fetch the BackupConfig instance
	config := &v1.Config{}
	isNotFound, isError := utils.IsObjectNotFound(ctx, r.Deps, req, config)
	if isNotFound {
		return ctrl.Result{}, nil
	} else if isError {
		return ctrl.Result{}, fmt.Errorf("failed to get Config")
	}

	// If deleting, allow it. Remove any tasks references and remove finalizer
	if config.DeletionTimestamp != nil {
		err := task_util.RemoveTasksByConfig(ctx, r.Deps, config)
		if err != nil {
			controllerutil.RemoveFinalizer(config, constants.ConfigFinalizer)
		}
		return ctrl.Result{}, err
	}

	now := metav1.Now()

	// If not initiliazed yet, add finalizer
	if config.Status.InitializedAt.IsZero() {
		controllerutil.AddFinalizer(config, constants.ConfigFinalizer)
		config.Status.InitializedAt = &now
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, r.Deps.Update(ctx, config)
	}

	// --------- Initialization done, processing ---------

	// If not initialized yet, run restic check + restic init and update the initialized --> rerun
	reconcile := false
	for _, repository := range config.Status.Repositories {
		if repository.Status.InitializedAt.IsZero() {
			logger.Infow("Repository not initialized yet", "repository", repository.Target)

			output, err := restic.ExecCheck(ctx, repository.Target, config.Spec.Env)
			if err == nil {
				logger.Infow("Repository checked", "output", output)
				repository.Status.InitializedAt = &now
				continue
			}

			reconcile = true

			output, err = restic.ExecInit(ctx, repository.Target, config.Spec.Env)
			if err == nil {
				logger.Infow("Repository initialized", "output", output)
				repository.Status.InitializedAt = &now
				continue
			}

			logger.Errorw("Failed to check/initialize repository", err)
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
		}
	}
	if reconcile {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, r.Deps.Update(ctx, config)
	}

	// All managed PVCs (which match selectors)
	managedPVCs := utils.GetPVCsForConfig(ctx, r.Deps, config)

	// Filter new and unclaimed PVCs for auto-restore processing
	newPVCs := utils.FilterNewPVCs(managedPVCs, r.Deps.Logger)
	unclaimedPVCs := utils.FilterUnclaimedPVCs(newPVCs, r.Deps.Logger)

	// Process new unclaimed PVCs for auto-restore. This has priority over other operations.
	if len(unclaimedPVCs) > 0 && config.Spec.AutoRestore {
		// @TODO: Allow for repository-level auto-restore override
		for _, pvc := range unclaimedPVCs {
			logger.Infow("New unclaimed PVC detected", "pvc", pvc.Name)

			args := restic.MakeRestoreArgs(restic.MakeArgsParams{
				Repositories: config.Spec.Repositories,
				Env:          config.Spec.Env,
				TargetPVC:    pvc,
				Annotation:   "", // Let Task figure out best restore point
			})

			// Create restore task for each PVC
			restoreTask := task_util.BuildTask(task_util.BuildTaskParams{
				PVC:      pvc,
				Env:      config.Spec.Env,
				Args:     args,
				TaskType: v1.TaskTypeRestoreAutomated,
			})

			if err := r.Deps.Create(ctx, &restoreTask); err != nil {
				logger.Errorw("Failed to create restore task for new PVC", "pvc", pvc.Name, "error", err)
				return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
			}

			logger.Infow("Created restore task for new PVC", "pvc", pvc.Name)

			// Mark PVC for auto-restore
			annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnAutoRestored: "true"})
			pvc.SetAnnotations(annotations)

			if err := r.Deps.Update(ctx, pvc); err != nil {
				logger.Errorw("Failed to mark PVC as auto-restored", "pvc", pvc.Name, "error", err)
				return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
			}
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// ------------ Process force-restore annotation on config ------------
	if config.Annotations[constants.AnnRestore] != "" {
		logger.Infow("Force reconcile annotation detected, removing ...")

		for _, pvc := range managedPVCs {
			// Prepare restore args
			args := restic.MakeRestoreArgs(restic.MakeArgsParams{
				Repositories: config.Spec.Repositories,
				Env:          config.Spec.Env,
				TargetPVC:    pvc,
				Annotation:   config.Annotations[constants.AnnRestore],
			})
			// Prepare restore task for pvc
			restoreTask := task_util.BuildTask(task_util.BuildTaskParams{
				PVC:      pvc,
				Env:      config.Spec.Env,
				Args:     args,
				TaskType: v1.TaskTypeRestoreManual,
			})
			// Create it
			if err := r.Deps.Create(ctx, &restoreTask); err != nil {
				logger.Errorw("Failed to create restore task for PVC", "pvc", pvc.Name, "error", err)
			} else {
				logger.Infow("Created restore task for PVC", "pvc", pvc.Name)
			}
		}

		// Remove annotation to avoid loops
		annotations := utils.MakeAnnotation(config.Annotations, map[string]string{constants.AnnRestore: ""})
		config.SetAnnotations(annotations)
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, r.Deps.Update(ctx, config)
	}

	// ------------ Process force-backup annotation on config ------------
	if config.Annotations[constants.AnnBackup] != "" {
		logger.Infow("Force backup annotation detected, removing ...")

		for _, pvc := range managedPVCs {
			args := restic.MakeBackupArgs(restic.MakeArgsParams{
				Repositories: config.Spec.Repositories,
				Env:          config.Spec.Env,
				TargetPVC:    pvc,
				Annotation:   config.Annotations[constants.AnnBackup],
			})

			backupTask := task_util.BuildTask(task_util.BuildTaskParams{
				PVC:      pvc,
				Env:      config.Spec.Env,
				Args:     args,
				TaskType: v1.TaskTypeBackupManual,
			})

			if err := r.Deps.Create(ctx, &backupTask); err != nil {
				logger.Errorw("Failed to create backup task for PVC", "pvc", pvc.Name, "error", err)
			} else {
				logger.Infow("Created backup task for PVC", "pvc", pvc.Name)
			}
		}

		// Remove annotation to avoid loops
		annotations := utils.MakeAnnotation(config.Annotations, map[string]string{constants.AnnBackup: ""})
		config.SetAnnotations(annotations)
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, r.Deps.Update(ctx, config)
	}

	// @TODO: Allow for repositories to be annotated for restore/backup

	for _, pvc := range managedPVCs {
		// ------------------  Check for restore annotation ------------------
		if pvc.Annotations[constants.AnnRestore] != "" {
			logger.Infow("Restore annotation detected on PVC", "pvc", pvc.Name)

			args := restic.MakeRestoreArgs(restic.MakeArgsParams{
				Repositories: config.Spec.Repositories,
				Env:          config.Spec.Env,
				TargetPVC:    pvc,
				Annotation:   pvc.Annotations[constants.AnnRestore],
			})

			restoreTask := task_util.BuildTask(task_util.BuildTaskParams{
				PVC:      pvc,
				Env:      config.Spec.Env,
				Args:     args,
				TaskType: v1.TaskTypeRestoreManual,
			})

			if err := r.Deps.Create(ctx, &restoreTask); err != nil {
				logger.Errorw("Failed to create restore task for PVC", "pvc", pvc.Name, "error", err)
			} else {
				logger.Infow("Created restore task for PVC", "pvc", pvc.Name)
			}

			annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnRestore: ""})
			pvc.SetAnnotations(annotations)
			if err := r.Deps.Update(ctx, pvc); err != nil {
				logger.Errorw("Failed to clear restore annotation on PVC", "pvc", pvc.Name, "error", err)
			} else {
				logger.Infow("Cleared restore annotation on PVC", "pvc", pvc.Name)
			}
		}

		// ------------------  Check for backup annotation ------------------
		if pvc.Annotations[constants.AnnBackup] != "" {
			logger.Infow("Backup annotation detected on PVC", "pvc", pvc.Name)

			args := restic.MakeBackupArgs(restic.MakeArgsParams{
				Repositories: config.Spec.Repositories,
				Env:          config.Spec.Env,
				TargetPVC:    pvc,
				Annotation:   pvc.Annotations[constants.AnnBackup],
			})

			backupTask := task_util.BuildTask(task_util.BuildTaskParams{
				PVC:      pvc,
				Env:      config.Spec.Env,
				Args:     args,
				TaskType: v1.TaskTypeBackupManual,
			})

			if err := r.Deps.Create(ctx, &backupTask); err != nil {
				logger.Errorw("Failed to create backup task for PVC", "pvc", pvc.Name, "error", err)
			} else {
				logger.Infow("Created backup task for PVC", "pvc", pvc.Name)
			}

			// Remove annotation to avoid loops
			annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnBackup: ""})
			pvc.SetAnnotations(annotations)

			if err := r.Deps.Update(ctx, pvc); err != nil {
				logger.Errorw("Failed to clear backup annotation on PVC", "pvc", pvc.Name, "error", err)
			} else {
				logger.Infow("Cleared backup annotation on PVC", "pvc", pvc.Name)
			}
		}
	}

	// If it has restore annotation, check for finalizer on PVC to avoid conflicts.
	// If not finalizer, create RestoreTask

	// If it has backup annotation, check for finalizer on PVC to avoid conflicts.
	// If not finalizer, create BackupTask

	// Process scheduled backups for each repository. Run one-by-one.
	for _, repository := range config.Status.Repositories {
		shouldUpdate := utils.ShouldPerformBackupFromRepository(repository)
		if !shouldUpdate {
			continue
		}

		for _, pvc := range managedPVCs {
			annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{
				// @TODO: Fix this
				constants.AnnBackup: repository.Target,
			})
			pvc.SetAnnotations(annotations)

			if err := r.Deps.Update(ctx, pvc); err != nil {
				logger.Errorw("Failed to mark PVC for backup", "pvc", pvc.Name, "error", err)
			} else {
				logger.Infow("Marked PVC for backup", "pvc", pvc.Name)
			}
		}

		repository.Status.LastScheduledBackupRun = &now

		// We have scheduled lot of backups, let them process before reconcile again
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, r.Deps.Update(ctx, config)
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, r.Deps.Status().Update(ctx, config)
}

func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Config{}).
		Owns(&v1.Task{}).
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
