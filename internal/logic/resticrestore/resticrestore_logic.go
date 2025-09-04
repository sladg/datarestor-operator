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

func cleanupRestoreJob(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) {
	log := deps.Logger.Named("[cleanupRestoreJob]")

	if restore.Status.Job.Name != "" {
		if err := utils.DeleteJob(ctx, deps, restore.Status.Job); err != nil {
			log.Warnw("Failed to delete job during cleanup", "error", err)
		}
	}
}

func manageRestoreWorkloads(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore, scaleDown bool) error {
	return utils.ManageWorkloadScaleForPVC(ctx, deps, restore.Spec.TargetPVC, restore, scaleDown)
}

func HandleRestoreUnknown(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	restore.Status.Phase = v1.PhasePending
	if err := deps.Status().Update(ctx, restore); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
}

func HandleRestorePending(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandlePending]")

	if err := utils.ValidateRestoreReferences(ctx, deps, restore); err != nil {
		log.Warnw("Failed to validate restore references", "error", err)
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, err.Error())
	}

	if err := utils.ValidateRestoreObjectsExist(ctx, deps, restore); err != nil {
		log.Warnw("Failed to validate restore objects exist", "error", err)
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, err.Error())
	}

	repositoryObj, err := utils.GetRepositoryForOperation(ctx, deps, restore.Spec.Repository.Namespace, restore.Spec.Repository.Name)
	if err != nil {
		log.Warnw("Failed to get repository for operation", "error", err)
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, nil
	}

	if utils.ContainsFinalizerWithRef(ctx, deps, restore.Spec.TargetPVC, constants.ResticRestoreFinalizer) {
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, nil
	}

	if err := utils.AddFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		log.Warnw("Failed to add finalizer", "error", err)
		return ctrl.Result{}, err
	}

	jobSpec := utils.BuildRestoreJobSpec(restore, repositoryObj)
	restore.Status.Phase = v1.PhaseRunning
	restore.Status.Job, _, err = utils.CreateResticJobWithOutput(ctx, deps, jobSpec, restore)
	if err != nil {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, err.Error())
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestoreRunning(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	finished, succeeded := utils.IsJobFinished(ctx, deps, restore.Status.Job)

	if !finished {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	restore.Status.CompletionTime = &metav1.Time{Time: metav1.Now().Time}

	if succeeded {
		restore.Status.Phase = v1.PhaseCompleted
	} else {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, "Restore job failed")
	}

	return ctrl.Result{}, deps.Status().Update(ctx, restore)
}

func HandleRestoreCompleted(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleCompleted]")

	cleanupRestoreJob(ctx, deps, restore)

	if err := manageRestoreWorkloads(ctx, deps, restore, false); err != nil {
		log.Errorw("Failed to scale up workloads during completion cleanup", err)
	}

	if err := utils.RemoveFinalizerWithRef(ctx, deps, restore.Spec.TargetPVC, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to remove restore finalizer from PVC during completion cleanup", err)
	}

	return ctrl.Result{}, nil
}

func HandleRestoreFailed(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleFailed]")

	if restore.Status.Job.Name != "" {
		if podLogs, _ := utils.GetJobLogs(ctx, deps, restore.Status.Job); podLogs != "" {
			restore.Status.Error = fmt.Sprintf("Restore job failed. Logs: %s", podLogs)
		}
		cleanupRestoreJob(ctx, deps, restore)
	}

	if err := manageRestoreWorkloads(ctx, deps, restore, false); err != nil {
		log.Errorw("Failed to scale up workloads during failure cleanup", err)
	}

	if err := utils.RemoveFinalizerWithRef(ctx, deps, restore.Spec.TargetPVC, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to remove restore finalizer from PVC during failure cleanup", err)
	}

	return ctrl.Result{}, deps.Status().Update(ctx, restore)
}

func HandleRestoreDeletion(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleDeletion]")

	if restore.Status.Phase == v1.PhaseUnknown || restore.Status.Phase == v1.PhaseRunning || restore.Status.Phase == v1.PhasePending {
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, nil
	}

	if restore.Status.Job.Name != "" {
		cleanupRestoreJob(ctx, deps, restore)
	}

	if err := manageRestoreWorkloads(ctx, deps, restore, false); err != nil {
		log.Errorw("Failed to scale up workloads during deletion cleanup", err)
	}

	if err := utils.RemoveFinalizerWithRef(ctx, deps, restore.Spec.TargetPVC, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to remove restore finalizer from PVC during deletion cleanup", err)
	}

	if err := utils.RemoveFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{}, nil
}
