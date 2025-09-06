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
)

func HandleBackupUnknown(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	backup.Status.Phase = v1.PhasePending
	backup.Status.StartTime = &metav1.Time{Time: metav1.Now().Time}

	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, deps.Status().Update(ctx, backup)
}

func HandleBackupPending(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandlePending]")
	deps.Logger = log

	if err := utils.ValidateBackupReferences(ctx, deps, backup); err != nil {
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, utils.SetOperationFailed(ctx, deps, backup, err)
	}

	if err := utils.ValidateBackupObjectsExist(ctx, deps, backup); err != nil {
		log.Warnw("Failed to validate backup objects exist", "error", err)
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, utils.SetOperationFailed(ctx, deps, backup, err)
	}

	repositoryObj, err := utils.GetRepositoryForOperation(ctx, deps, backup.Spec.Repository.Namespace, backup.Spec.Repository.Name)
	if err != nil {
		if err.Error() == "repository not ready" {
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
		}
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}
	if repositoryObj.Status.Phase != v1.PhaseCompleted {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if err := utils.AddFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer); err != nil {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, backup, err)
	}

	if err := utils.AddFinalizerWithRef(ctx, deps, backup.Spec.SourcePVC, constants.ResticBackupFinalizer); err != nil {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, backup, err)
	}

	if utils.ContainsFinalizerWithRef(ctx, deps, backup.Spec.SourcePVC, constants.ResticBackupFinalizer) {
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, nil
	}

	jobSpec := utils.BuildBackupJobSpec(backup, repositoryObj)
	backup.Status.Phase = v1.PhaseRunning
	backup.Status.Job, _, err = utils.CreateResticJobWithOutput(ctx, deps, jobSpec, backup)
	if err != nil {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, backup, err)
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

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, deps.Status().Update(ctx, backup)
}

func HandleBackupCompleted(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleCompleted]")
	deps.Logger = log

	// Already terminal stage
	if backup.Status.CompletionTime != nil {
		return ctrl.Result{}, nil
	}

	backup.Status.CompletionTime = &metav1.Time{Time: metav1.Now().Time}
	utils.CleanupJob(ctx, deps, corev1.ObjectReference{Name: backup.Status.Job.Name, Namespace: backup.Status.Job.Namespace})

	if err := utils.CleanupBeforeFinale(ctx, deps, backup.Spec.SourcePVC, backup, constants.ResticBackupFinalizer); err != nil {
		log.Warnw("Failed to cleanup before finalizer during completion cleanup", "error", err)
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, err
	}

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

	podLogs := utils.CleanupJobWithLogs(ctx, deps, corev1.ObjectReference{Name: backup.Status.Job.Name, Namespace: backup.Status.Job.Namespace})
	backup.Status.Error = fmt.Sprintf("Backup job failed. Logs: %s", podLogs)

	if err := utils.CleanupBeforeFinale(ctx, deps, backup.Spec.SourcePVC, backup, constants.ResticBackupFinalizer); err != nil {
		log.Warnw("Failed to cleanup before finalizer during failure cleanup", "error", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, deps.Status().Update(ctx, backup)
}

func HandleBackupDeletion(ctx context.Context, deps *utils.Dependencies, backup *v1.ResticBackup) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleDeletion]")
	deps.Logger = log

	utils.CleanupJob(ctx, deps, backup.Status.Job)

	if backup.Status.Job.Name != "" {
		if err := utils.DeleteJob(ctx, deps, backup.Status.Job); err != nil {
			log.Warnw("Failed to delete job during cleanup", "error", err)
		}
	}

	if err := utils.CleanupBeforeFinale(ctx, deps, backup.Spec.SourcePVC, backup, constants.ResticBackupFinalizer); err != nil {
		log.Warnw("Failed to cleanup before finalizer during deletion cleanup", "error", err)
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, err
	}

	return ctrl.Result{}, utils.RemoveFinalizer(ctx, deps, backup, constants.ResticBackupFinalizer)
}
