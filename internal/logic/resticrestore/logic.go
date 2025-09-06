package resticrestore

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/logic/resticrepository"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func HandleRestoreUnknown(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	restore.Status.Phase = v1.PhasePending
	restore.Status.StartTime = &metav1.Time{Time: metav1.Now().Time}

	// @TODO: Remove annotation from PVC so that backup is not re-triggered

	if err := utils.SetOwnFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestorePending(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRestorePending]")
	deps.Logger = log

	ready, repositoryObj := resticrepository.CheckRepositoryReady(ctx, deps, restore.Spec.Repository)
	locked := utils.CheckFinalizerOnPVC(ctx, deps, restore.Spec.TargetPVC, constants.ResticRestoreFinalizer)
	if !ready || locked {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	utils.LockUnlockForOperation(ctx, deps, utils.PrepareFinalizeParams{
		Owner:     restore,
		PVC:       &restore.Spec.TargetPVC,
		ScaleDown: true,
	})

	args := GetRestoreArgs(ctx, deps, restore)
	args = append(args, restore.Spec.Args...)

	var err error
	jobSpec := utils.BuildRestoreJobSpec(restore, repositoryObj, args)
	restore.Status.Phase = v1.PhaseRunning
	restore.Status.Job, _, err = utils.CreateResticJobWithOutput(ctx, deps, jobSpec, restore)

	if err != nil {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, err)
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestoreRunning(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRestoreRunning]")
	deps.Logger = log

	finished, succeeded := utils.IsJobFinished(ctx, deps, restore.Status.Job)

	if !finished {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		restore.Status.Phase = v1.PhaseCompleted
		return ctrl.Result{}, deps.Status().Update(ctx, restore)
	} else {
		return ctrl.Result{}, utils.SetOperationFailed(ctx, deps, restore, fmt.Errorf("restore job failed"))
	}
}

func HandleRestoreCompleted(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRestoreCompleted]")
	deps.Logger = log

	// Already terminal stage
	if restore.Status.CompletionTime != nil {
		return ctrl.Result{}, nil
	}

	utils.CleanupJob(ctx, deps, corev1.ObjectReference{Name: restore.Status.Job.Name, Namespace: restore.Status.Job.Namespace})

	utils.LockUnlockForOperation(ctx, deps, utils.PrepareFinalizeParams{
		Owner:     restore,
		PVC:       &restore.Spec.TargetPVC,
		ScaleDown: false,
	})

	restore.Status.CompletionTime = &metav1.Time{Time: metav1.Now().Time}
	restore.Status.Job = corev1.ObjectReference{}

	return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestoreFailed(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRestoreFailed]")
	deps.Logger = log

	// Already terminal stage
	if restore.Status.FailedTime != nil {
		return ctrl.Result{}, nil
	}

	utils.CleanupJob(ctx, deps, corev1.ObjectReference{Name: restore.Status.Job.Name, Namespace: restore.Status.Job.Namespace})

	utils.LockUnlockForOperation(ctx, deps, utils.PrepareFinalizeParams{
		Owner:     restore,
		PVC:       &restore.Spec.TargetPVC,
		ScaleDown: false,
	})

	restore.Status.FailedTime = &metav1.Time{Time: metav1.Now().Time}
	restore.Status.Error = fmt.Sprintf("Restore job failed. Logs: %s", "NOT IMPLEMENTED")
	restore.Status.Job = corev1.ObjectReference{}

	return ctrl.Result{RequeueAfter: constants.LongerRequeueInterval}, deps.Status().Update(ctx, restore)
}

func HandleRestoreDeletion(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("[HandleRestoreDeletion]")
	deps.Logger = log

	if utils.Contains(constants.ActivePhases, restore.Status.Phase) {
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
	}

	log.Info("Handling restore deletion. Everything should be cleaned up.")

	return ctrl.Result{}, utils.RemoveOwnFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer)
}
