package resticrestore

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func HandleRestorePending(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-pending")

	repositoryObj, err := utils.GetResource[*v1.ResticRepository](ctx, deps.Client, restore.Spec.Repository.Namespace, restore.Spec.Repository.Name)
	if err != nil {
		log.Errorw("Failed to get repository", "error", err)
		return ctrl.Result{}, err
	}

	err = utils.AddFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer)
	if err != nil {
		log.Errorw("add finalizer failed", "error", err)
		return ctrl.Result{}, err
	}

	// Check repository is ready
	if repositoryObj.Status.Phase != v1.PhaseCompleted {
		log.Debug("Repository not ready, requeueing")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	stopPods := utils.ShouldStopPods(repositoryObj.Spec.BackupConfig)

	if stopPods {
		if err := utils.ManageWorkloadScaleForPVC(ctx, deps, restore.Spec.TargetPVC, restore, true); err != nil {
			log.Errorw("Failed to scale down workloads", "error", err)
			return ctrl.Result{}, err
		}
	}

	jobSpec := utils.BuildRestoreJobSpec(restore, repositoryObj)
	restore.Status.Phase = v1.PhaseRunning
	restore.Status.Job, _, err = utils.CreateResticJobWithOutput(ctx, deps, jobSpec, restore)
	if err != nil {
		log.Errorw("Failed to create restore job", "error", err)
		restore.Status.Phase = v1.PhaseFailed
		restore.Status.Error = err.Error()
		if err := deps.Update(ctx, restore); err != nil {
			log.Errorw("Failed to update restore status", "error", err)
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, err
	}

	err = deps.Status().Update(ctx, restore)
	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
}

func HandleRestoreRunning(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore")
	log.Info("Handling running restore")

	finished, succeeded := utils.IsJobFinished(ctx, deps, restore.Status.Job)

	if !finished {
		log.Debug("Restore job is still running")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		log.Info("Restore job succeeded. Moving to Completed phase.")
		restore.Status.Phase = v1.PhaseCompleted
	} else {
		log.Errorw("Restore job failed. Moving to Failed phase.")
		restore.Status.Phase = v1.PhaseFailed

		// @TODO: Get errors from pod logs / job
		restore.Status.Error = "Restore job failed"
	}

	restore.Status.CompletionTime = &metav1.Time{Time: metav1.Now().Time}
	err := deps.Status().Update(ctx, restore)
	return ctrl.Result{}, err

}

func HandleRestoreCompleted(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-completed")
	log.Info("Handling completed restore")

	if restore.Status.Job.Name == "" {
		return ctrl.Result{}, nil
	}

	if err := utils.DeleteJob(ctx, deps, restore.Status.Job); err != nil {
		log.Errorw("Failed to clean up completed restore job", "error", err)
	} else {
		log.Info("Successfully cleaned up completed restore job")
	}

	if err := utils.ManageWorkloadScaleForPVC(ctx, deps, restore.Spec.TargetPVC, restore, false); err != nil {
		log.Errorw("Failed to scale up workloads after successful restore", "error", err)
	}

	return ctrl.Result{}, nil
}

func HandleRestoreFailed(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-failed")
	log.Info("Handling failed restore")

	if restore.Status.Job.Name == "" {
		return ctrl.Result{}, nil
	}

	podLogs, _ := utils.GetJobLogs(ctx, deps, restore.Status.Job)

	restore.Status.Phase = v1.PhaseFailed

	// @TODO: Get errors from pod logs / job
	restore.Status.Error = fmt.Sprintf("Reason: %s, Message: %s, Logs: %s", "Restore job failed", "Restore job failed", podLogs)

	log.Errorw("Restore job failed", restore.Status.Error)

	// Update status with the log message
	if err := deps.Status().Update(ctx, restore); err != nil {
		log.Errorw("Failed to update restore status with failure logs", "error", err)
	}

	// Clean up the failed job
	if err := utils.DeleteJob(ctx, deps, restore.Status.Job); err != nil {
		log.Errorw("Failed to clean up failed restore job", "error", err)
	} else {
		log.Info("Successfully cleaned up failed restore job")
	}

	if err := utils.ManageWorkloadScaleForPVC(ctx, deps, restore.Spec.TargetPVC, restore, false); err != nil {
		log.Errorw("Failed to scale up workloads after failed restore", "error", err)
	}

	return ctrl.Result{}, nil
}

func HandleRestoreDeletion(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-deletion")

	// Block deletion if the restore is in an active phase
	// Once the phase transitions to completed or failed, the deletion is allowed - it will handle scaling up the workloads
	if restore.Status.Phase == v1.PhaseUnknown || restore.Status.Phase == v1.PhaseRunning || restore.Status.Phase == v1.PhasePending {
		log.Info("Deletion is blocked because the restore is in an active phase", "phase", restore.Status.Phase)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// Remove our finalizer
	if err := utils.RemoveFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to remove finalizer", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{}, nil
}
