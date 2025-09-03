package resticbackup

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func HandleBackupPending(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-pending")

	err := utils.AddFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer)
	if err != nil {
		log.Errorw("Failed to add finalizer", "error", err)
		return ctrl.Result{}, err
	}

	// Check repository is ready
	if backup.Spec.Repository.Status.Phase != v1.PhaseCompleted {
		log.Debug("Repository not ready, requeueing")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	stopPods := utils.ShouldStopPods(backup.Spec.Repository.Spec.BackupConfig)
	if stopPods {
		if err := utils.ManageWorkloadScaleForPVC(ctx, deps, backup.Spec.SourcePVC, backup, true); err != nil {
			log.Errorw("Failed to scale down workloads", "error", err)
			return ctrl.Result{}, err
		}
	}

	jobSpec := utils.BuildBackupJobSpec(backup, backup.Spec.Repository)
	backup.Status.Phase = v1.PhaseRunning
	backup.Status.Job, _, err = utils.CreateResticJobWithOutput(ctx, deps, jobSpec, backup)
	if err != nil {
		log.Errorw("Failed to create backup job", "error", err)
		backup.Status.Phase = v1.PhaseFailed
		backup.Status.Error = err.Error()
		return ctrl.Result{}, err
	}

	err = deps.Status().Update(ctx, backup)
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err

}

func HandleBackupRunning(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-running")
	log.Info("Handling running backup")

	finished, succeeded := utils.IsJobFinished(backup.Status.Job)

	if !finished {
		log.Debug("Backup job is still running")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		log.Info("Restore job succeeded. Moving to Completed phase.")
		backup.Status.Phase = v1.PhaseCompleted
	} else {
		log.Errorw("Backup job failed. Moving to Failed phase.")
		backup.Status.Phase = v1.PhaseFailed
		backup.Status.Error = backup.Status.Job.Status.Conditions[0].Message
	}

	backup.Status.CompletionTime = &metav1.Time{Time: metav1.Now().Time}
	err := deps.Status().Update(ctx, backup)
	return ctrl.Result{}, err
}

func HandleBackupCompleted(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-completed")
	log.Info("Handling completed backup")

	if backup.Status.Job == nil {
		return ctrl.Result{}, nil
	}

	if err := deps.Delete(ctx, backup.Status.Job); err != nil {
		log.Errorw("Failed to clean up completed backup job", "error", err)
	} else {
		log.Info("Successfully cleaned up completed backup job")
	}

	if err := utils.ManageWorkloadScaleForPVC(ctx, deps, backup.Spec.SourcePVC, backup, false); err != nil {
		log.Errorw("Failed to scale up workloads on completion", "error", err)
	}

	return ctrl.Result{}, nil
}

func HandleBackupFailed(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-failed")
	log.Info("Handling failed backup")

	if backup.Status.Job == nil {
		return ctrl.Result{}, nil
	}

	podLogs, _ := utils.GetJobLogs(ctx, deps, backup.Status.Job)

	backup.Status.Error = fmt.Sprintf("Reason: %s, Message: %s, Logs: %s", backup.Status.Job.Status.Conditions[0].Reason, backup.Status.Job.Status.Conditions[0].Message, podLogs)
	log.Errorw("Backup job failed", backup.Status.Error)

	if err := deps.Status().Update(ctx, backup); err != nil {
		log.Errorw("Failed to update backup status with failure logs", "error", err)
	}

	if err := deps.Delete(ctx, backup.Status.Job); err != nil {
		log.Errorw("Failed to clean up failed backup job", "error", err)
	} else {
		log.Info("Successfully cleaned up failed backup job")
	}

	if err := utils.ManageWorkloadScaleForPVC(ctx, deps, backup.Spec.SourcePVC, backup, false); err != nil {
		log.Errorw("Failed to scale up workloads after failed backup", "error", err)
	}

	return ctrl.Result{}, nil
}

func HandleBackupDeletion(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("backup-deletion")

	// Block deletion if the restore is in an active phase
	// Once the phase transitions to completed or failed, the deletion is allowed - it will handle scaling up the workloads
	if backup.Status.Phase == v1.PhaseUnknown || backup.Status.Phase == v1.PhaseRunning || backup.Status.Phase == v1.PhasePending {
		log.Info("Deletion is blocked because the backup is in an active phase", "phase", backup.Status.Phase)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// Remove our finalizer
	if err := utils.RemoveFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer); err != nil {
		log.Errorw("Failed to remove finalizer", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{}, nil
}
