package resticbackup

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/sladg/datarestor-operator/internal/logic/resticrepository"
)

func HandleBackupUnknown(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	backup.Status.Phase = v1.PhasePending
	backup.Status.StartTime = &metav1.Time{Time: metav1.Now().Time}

	// @TODO: Remove annotation from PVC so that backup is not re-triggered

	if err := utils.SetOwnFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, deps.Status().Update(ctx, backup)
}

func HandleBackupPending(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleBackupPending]")
	deps.Logger = log

	ready, repositoryObj := resticrepository.CheckRepositoryReady(ctx, deps, backup.Spec.Repository)
	locked := utils.CheckFinalizerOnPVC(ctx, deps, backup.Spec.SourcePVC, constants.ResticBackupFinalizer)
	if !ready || locked {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	utils.LockUnlockForOperation(ctx, deps, utils.PrepareFinalizeParams{
		Owner:     backup,
		PVC:       &backup.Spec.SourcePVC,
		ScaleDown: true,
	})

	args := append([]string{}, backup.Spec.Args...)
	var err error

	jobSpec := utils.BuildBackupJobSpec(repositoryObj, backup, args)
	backup.Status.Phase = v1.PhaseRunning
	backup.Status.Job, _, err = utils.CreateResticJobWithOutput(ctx, deps, jobSpec, backup)

	if err != nil {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, backup, err)
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, deps.Status().Update(ctx, backup)
}

func HandleBackupRunning(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleBackupRunning]")
	deps.Logger = log

	finished, succeeded := utils.IsJobFinished(ctx, deps, backup.Status.Job)

	if !finished {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		backup.Status.Phase = v1.PhaseCompleted
		return ctrl.Result{}, deps.Status().Update(ctx, backup)
	} else {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, backup, fmt.Errorf("backup job failed"))
	}
}

func HandleBackupCompleted(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleCompleted]")
	deps.Logger = log

	// Already terminal stage
	if backup.Status.CompletionTime != nil {
		return ctrl.Result{}, nil
	}

	utils.CleanupJob(ctx, deps, corev1.ObjectReference{Name: backup.Status.Job.Name, Namespace: backup.Status.Job.Namespace})

	utils.LockUnlockForOperation(ctx, deps, utils.PrepareFinalizeParams{
		Owner:     backup,
		PVC:       &backup.Spec.SourcePVC,
		ScaleDown: false,
	})

	backup.Status.CompletionTime = &metav1.Time{Time: metav1.Now().Time}
	backup.Status.Job = corev1.ObjectReference{}

	return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, deps.Status().Update(ctx, backup)
}

func HandleBackupFailed(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleFailed]")
	deps.Logger = log

	// Already terminal stage
	if backup.Status.FailedTime != nil {
		return ctrl.Result{}, nil
	}

	backup.Status.FailedTime = &metav1.Time{Time: metav1.Now().Time}

	utils.CleanupJob(ctx, deps, corev1.ObjectReference{Name: backup.Status.Job.Name, Namespace: backup.Status.Job.Namespace})
	backup.Status.Error = fmt.Sprintf("Backup job failed. Logs: %s", "NOT IMPLEMENTED")

	utils.LockUnlockForOperation(ctx, deps, utils.PrepareFinalizeParams{
		Owner:     backup,
		PVC:       &backup.Spec.SourcePVC,
		ScaleDown: false,
	})

	return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, deps.Status().Update(ctx, backup)
}

func HandleBackupDeletion(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleDeletion]")
	deps.Logger = log

	if utils.Contains(constants.ActivePhases, backup.Status.Phase) {
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
	}

	log.Info("Handling backup deletion. Everything should be cleaned up.")

	return ctrl.Result{}, utils.RemoveOwnFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer)
}
