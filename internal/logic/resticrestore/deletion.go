package logic

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HandleRestoreDeletion handles the deletion of a ResticRestore resource.
func HandleRestoreDeletion(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-deletion")

	// Block deletion if the restore is in an active phase
	if restore.Status.Phase == v1.PhaseRunning || restore.Status.Phase == v1.PhasePending {
		log.Info("Deletion is blocked because the restore is in an active phase", "phase", restore.Status.Phase)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	// Clean up by scaling up any workloads that might have been scaled down.
	if pvc, err := utils.GetResource[corev1.PersistentVolumeClaim](ctx, deps.Client, restore.Spec.TargetPVC.Namespace, restore.Spec.TargetPVC.Name, log); err != nil {
		if !errors.IsNotFound(err) {
			log.Errorw("Failed to get PVC for scaling up workloads on deletion", "error", err)
			return ctrl.Result{}, err
		}
		log.Warnw("PVC not found, cannot scale up workloads", "pvc", restore.Spec.TargetPVC.Name)
	} else {
		if err := utils.ManageWorkloadScaleForPVC(ctx, deps, pvc, restore, false); err != nil {
			log.Errorw("Failed to scale up workloads on deletion", "error", err)
			// Don't requeue, just log and continue cleanup.
		}
	}

	// Remove our finalizer
	if err := utils.RemoveFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to remove finalizer", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{}, nil
}
