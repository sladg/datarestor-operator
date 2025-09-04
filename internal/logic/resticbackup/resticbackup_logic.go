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

func HandleBackupUnknown(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	backup.Status.Phase = v1.PhasePending
	if err := deps.Status().Update(ctx, backup); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
}

func HandleBackupPending(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandlePending]")
	if err := utils.ValidateBackupReferences(ctx, deps, backup); err != nil {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, backup, err.Error())
	}

	if err := utils.ValidateBackupObjectsExist(ctx, deps, backup); err != nil {
		log.Warnw("Failed to validate backup objects exist", "error", err)
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, backup, err.Error())
	}

	repositoryObj, err := utils.GetRepositoryForOperation(ctx, deps, backup.Spec.Repository.Namespace, backup.Spec.Repository.Name)
	if err != nil {
		if err.Error() == "repository not ready" {
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if err := utils.AddFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer); err != nil {
		return ctrl.Result{}, err
	}

	if err := utils.AddFinalizerWithRef(ctx, deps, backup.Spec.SourcePVC, constants.ResticBackupFinalizer); err != nil {
		return ctrl.Result{}, err
	}

	if repositoryObj.Status.Phase != v1.PhaseCompleted {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if utils.ContainsFinalizerWithRef(ctx, deps, backup.Spec.SourcePVC, constants.ResticBackupFinalizer) {
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, nil
	}

	jobSpec := utils.BuildBackupJobSpec(backup, repositoryObj)
	backup.Status.Phase = v1.PhaseRunning
	backup.Status.StartTime = &metav1.Time{Time: metav1.Now().Time}
	backup.Status.Job, _, err = utils.CreateResticJobWithOutput(ctx, deps, jobSpec, backup)
	if err != nil {
		backup.Status.Phase = v1.PhaseFailed
		backup.Status.Error = err.Error()
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, deps.Status().Update(ctx, backup)
}

func HandleBackupRunning(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	finished, succeeded := utils.IsJobFinished(ctx, deps, backup.Status.Job)

	if !finished {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	completionTime := metav1.Now()
	backup.Status.CompletionTime = &metav1.Time{Time: completionTime.Time}

	if backup.Status.StartTime != nil {
		duration := completionTime.Sub(backup.Status.StartTime.Time)
		backup.Status.Duration = metav1.Duration{Duration: duration}
	}

	if succeeded {
		backup.Status.Phase = v1.PhaseCompleted
	} else {
		backup.Status.Phase = v1.PhaseFailed
		backup.Status.Error = "Backup job failed"
	}

	return ctrl.Result{}, deps.Status().Update(ctx, backup)
}

func HandleBackupCompleted(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleCompleted]")

	if err := utils.DeleteJob(ctx, deps, backup.Status.Job); err != nil {
		log.Warnw("Failed to delete job during cleanup", "error", err)
	}
	if err := utils.ManageWorkloadScaleForPVC(ctx, deps, backup.Spec.SourcePVC, backup, false); err != nil {
		log.Warnw("Failed to manage workload scale during cleanup", "error", err)
	}
	if err := utils.RemoveFinalizerWithRef(ctx, deps, backup.Spec.SourcePVC, constants.ResticBackupFinalizer); err != nil {
		log.Warnw("Failed to remove finalizer during cleanup", "error", err)
	}
	return ctrl.Result{}, nil
}

func HandleBackupFailed(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleFailed]")

	if backup.Status.Job.Name != "" {
		if podLogs, _ := utils.GetJobLogs(ctx, deps, backup.Status.Job); podLogs != "" {
			backup.Status.Error = fmt.Sprintf("Backup job failed. Logs: %s", podLogs)
		}
		if err := utils.DeleteJob(ctx, deps, backup.Status.Job); err != nil {
			log.Warnw("Failed to delete job during cleanup", "error", err)
		}
	}

	if err := utils.ManageWorkloadScaleForPVC(ctx, deps, backup.Spec.SourcePVC, backup, false); err != nil {
		log.Warnw("Failed to manage workload scale during cleanup", "error", err)
	}
	if err := utils.RemoveFinalizerWithRef(ctx, deps, backup.Spec.SourcePVC, constants.ResticBackupFinalizer); err != nil {
		log.Warnw("Failed to remove finalizer during cleanup", "error", err)
	}

	return ctrl.Result{}, deps.Status().Update(ctx, backup)
}

func HandleBackupDeletion(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleDeletion]")

	if backup.Status.Phase == v1.PhaseUnknown || backup.Status.Phase == v1.PhaseRunning || backup.Status.Phase == v1.PhasePending {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if backup.Status.Job.Name != "" {
		if err := utils.DeleteJob(ctx, deps, backup.Status.Job); err != nil {
			log.Warnw("Failed to delete job during cleanup", "error", err)
		}
	}

	if err := utils.ManageWorkloadScaleForPVC(ctx, deps, backup.Spec.SourcePVC, backup, false); err != nil {
		log.Warnw("Failed to manage workload scale during cleanup", "error", err)
	}
	if err := utils.RemoveFinalizerWithRef(ctx, deps, backup.Spec.SourcePVC, constants.ResticBackupFinalizer); err != nil {
		log.Warnw("Failed to remove finalizer during cleanup", "error", err)
	}

	if err := utils.RemoveFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer); err != nil {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{}, nil
}
