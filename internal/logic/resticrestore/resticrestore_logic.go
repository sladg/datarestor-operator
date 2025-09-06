package resticrestore

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

func HandleRestoreUnknown(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	restore.Status.Phase = v1.PhasePending
	restore.Status.StartTime = &metav1.Time{Time: metav1.Now().Time}

	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestorePending(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandlePending]")
	deps.Logger = log

	if err := utils.ValidateRestoreReferences(ctx, deps, restore); err != nil {
		log.Warnw("Failed to validate restore references", "error", err)
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, utils.SetOperationFailed(ctx, deps, restore, err)
	}

	if err := utils.ValidateRestoreObjectsExist(ctx, deps, restore); err != nil {
		log.Warnw("Failed to validate restore objects exist", "error", err)
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, err)
	}

	repositoryObj, err := utils.GetRepositoryForOperation(ctx, deps, restore.Spec.Repository.Namespace, restore.Spec.Repository.Name)
	if err != nil {
		log.Warnw("Failed to get repository for operation", "error", err)
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, nil
	}
	if repositoryObj.Status.Phase != v1.PhaseCompleted {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if utils.ContainsFinalizerWithRef(ctx, deps, restore.Spec.TargetPVC, constants.ResticRestoreFinalizer) {
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, nil
	}

	if err := utils.AddFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		log.Warnw("Failed to add finalizer", "error", err)
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, err)
	}

	jobSpec := utils.BuildRestoreJobSpec(restore, repositoryObj)
	restore.Status.Phase = v1.PhaseRunning
	restore.Status.Job, _, err = utils.CreateResticJobWithOutput(ctx, deps, jobSpec, restore)
	if err != nil {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, err)
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestoreRunning(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	finished, succeeded := utils.IsJobFinished(ctx, deps, restore.Status.Job)

	if !finished {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		restore.Status.Phase = v1.PhaseCompleted
	} else {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, fmt.Errorf("Restore job failed"))
	}

	return ctrl.Result{}, deps.Status().Update(ctx, restore)
}

func HandleRestoreCompleted(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleCompleted]")
	deps.Logger = log

	// Already terminal stage
	if restore.Status.CompletionTime != nil {
		return ctrl.Result{}, nil
	}

	restore.Status.CompletionTime = &metav1.Time{Time: metav1.Now().Time}
	utils.CleanupJob(ctx, deps, corev1.ObjectReference{Name: restore.Status.Job.Name, Namespace: restore.Status.Job.Namespace})

	if err := utils.CleanupBeforeFinale(ctx, deps, restore.Spec.TargetPVC, restore, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to cleanup before finalizer during completion cleanup", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestoreFailed(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleFailed]")
	deps.Logger = log

	// Already terminal stage
	if restore.Status.FailedTime != nil {
		return ctrl.Result{}, nil
	}

	restore.Status.FailedTime = &metav1.Time{Time: metav1.Now().Time}

	podLogs := utils.CleanupJobWithLogs(ctx, deps, corev1.ObjectReference{Name: restore.Status.Job.Name, Namespace: restore.Status.Job.Namespace})
	restore.Status.Error = fmt.Sprintf("Restore job failed. Logs: %s", podLogs)

	if err := utils.CleanupBeforeFinale(ctx, deps, restore.Spec.TargetPVC, restore, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to cleanup before finalizer during failure cleanup", err)
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestoreDeletion(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleDeletion]")
	deps.Logger = log

	if err := utils.CleanupBeforeFinale(ctx, deps, restore.Spec.TargetPVC, restore, constants.ResticRestoreFinalizer); err != nil {
		return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, err
	}

	return ctrl.Result{}, utils.RemoveFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer)
}
