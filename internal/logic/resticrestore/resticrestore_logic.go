package logic

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// HandleResticRestoreDeletion handles the deletion logic for a ResticRestore.
func HandleResticRestoreDeletion(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore")
	log.Info("Starting deletion logic for ResticRestore")

	// Our finalizer doesn't do anything specific on deletion,
	// as the restore job is owned by the CR and will be garbage collected.
	// Workload scaling is handled by the workload finalizer, not this one.

	if err := utils.RemoveFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	log.Info("Completed deletion logic for ResticRestore")
	return ctrl.Result{}, nil
}

// HandleRestorePending handles the logic when a ResticRestore is in the Pending phase.
func HandleRestorePending(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore")
	log.Info("Handling pending restore")

	// 1. Add finalizer
	if err := utils.AddFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to add finalizer", "error", err)
		return ctrl.Result{}, err
	}

	// 2. Scale down workloads before restoring
	pvc, err := utils.GetResource[corev1.PersistentVolumeClaim](ctx, deps.Client, restore.Namespace, restore.Spec.TargetPVC.Name, log)
	if err != nil {
		return ctrl.Result{}, err
	}
	if err := utils.ManageWorkloadScaleForPVC(ctx, deps, pvc, restore, true); err != nil {
		log.Errorw("Failed to scale down workloads", "error", err)
		return ctrl.Result{}, err
	}

	// 3. Create the restore job
	job, err := createRestoreJob(ctx, deps, restore)
	if err != nil {
		log.Errorw("Failed to create restore job", "error", err)
		return ctrl.Result{}, err
	}

	// 4. Update status to Running
	return updateResticRestoreStatus(ctx, deps, restore, v1.PhaseRunning, metav1.Now(), job.Name)
}

// HandleRestoreRunning handles the logic when a ResticRestore is in the Running phase.
func HandleRestoreRunning(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore")
	log.Info("Handling running restore")

	// 1. Get the Kubernetes Job
	job, err := utils.GetResource[batchv1.Job](ctx, deps.Client, restore.Namespace, restore.Status.Job.JobRef.Name, log)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Errorw("Restore job not found, assuming failure and moving to Failed phase", "jobName", restore.Status.Job.JobRef.Name, "error", err)
			return updateResticRestoreStatus(ctx, deps, restore, v1.PhaseFailed, metav1.Now(), restore.Status.Job.JobRef.Name)
		}
		return ctrl.Result{}, err
	}

	// 2. Check job status and update restore status accordingly
	return processRestoreJobStatus(ctx, deps, restore, job)
}

// HandleRestoreCompleted handles the logic when a ResticRestore is in the Completed phase.
func HandleRestoreCompleted(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-completed")
	log.Info("Handling completed restore")

	// Clean up the job since it's completed successfully
	jobName := restore.Status.Job.JobRef.Name
	job, err := utils.GetResource[batchv1.Job](ctx, deps.Client, restore.Spec.TargetPVC.Namespace, jobName, log)
	if err == nil { // Job exists, so delete it
		if err := deps.Client.Delete(ctx, job); err != nil {
			log.Errorw("Failed to clean up completed restore job", "error", err)
		} else {
			log.Info("Successfully cleaned up completed restore job")
		}
	} else if !errors.IsNotFound(err) {
		// Error already logged by GetResource
	}

	// Scale up workloads
	pvc, err := utils.GetResource[corev1.PersistentVolumeClaim](ctx, deps.Client, restore.Namespace, restore.Spec.TargetPVC.Name, log)
	if err != nil {
		// Error logged by GetResource, nothing more to do
	} else if err := utils.ManageWorkloadScaleForPVC(ctx, deps, pvc, restore, false); err != nil {
		log.Errorw("Failed to scale up workloads after successful restore", "error", err)
	}

	return ctrl.Result{}, nil
}

// HandleRestoreFailed handles the logic when a ResticRestore is in the Failed phase.
func HandleRestoreFailed(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-failed")
	log.Info("Handling failed restore")

	// Get the failed job to extract logs
	jobName := restore.Status.Job.JobRef.Name
	job, err := utils.GetResource[batchv1.Job](ctx, deps.Client, restore.Spec.TargetPVC.Namespace, jobName, log)
	if err == nil { // Job exists
		// Attempt to get logs from the failed pod for better error reporting
		podLogs, logErr := utils.GetJobLogs(ctx, deps, job)
		if logErr != nil {
			log.Errorw("Failed to get logs from failed restore job pod", "error", logErr)
		}
		log.Errorw("Restore job failed", "reason", job.Status.Conditions[0].Reason, "message", job.Status.Conditions[0].Message, "logs", podLogs)
		restore.Status.Error = fmt.Sprintf("Reason: %s, Message: %s, Logs: %s", job.Status.Conditions[0].Reason, job.Status.Conditions[0].Message, podLogs)

		// Update status with the log message
		if err := deps.Client.Status().Update(ctx, restore); err != nil {
			log.Errorw("Failed to update restore status with failure logs", "error", err)
		}

		// Clean up the failed job
		if err := deps.Client.Delete(ctx, job); err != nil {
			log.Errorw("Failed to clean up failed restore job", "error", err)
		} else {
			log.Info("Successfully cleaned up failed restore job")
		}
	} else if !errors.IsNotFound(err) {
		// Error already logged by GetResource
	}

	// Attempt to scale up workloads even if the restore failed
	pvc, err := utils.GetResource[corev1.PersistentVolumeClaim](ctx, deps.Client, restore.Namespace, restore.Spec.TargetPVC.Name, log)
	if err != nil {
		// Error logged by GetResource, nothing more to do
	} else if err := utils.ManageWorkloadScaleForPVC(ctx, deps, pvc, restore, false); err != nil {
		log.Errorw("Failed to scale up workloads after failed restore", "error", err)
	}

	return ctrl.Result{}, nil
}

// createRestoreJob creates a new restore job.
func createRestoreJob(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) (*batchv1.Job, error) {
	jobSpec, err := utils.BuildRestoreJobSpec(restore)
	if err != nil {
		return nil, err
	}

	job, _, err := utils.CreateResticJobWithOutput(ctx, deps, jobSpec, restore)
	if err != nil {
		return nil, fmt.Errorf("failed to create restic job: %w", err)
	}

	deps.Logger.Infow("Restore job created", "jobName", job.Name)
	return job, nil
}

// processRestoreJobStatus checks the status of the Kubernetes Job and updates the ResticRestore status.
func processRestoreJobStatus(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore, job *batchv1.Job) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-job-status")
	finished, succeeded := utils.IsJobFinished(job)

	if !finished {
		log.Debug("Restore job is still running")
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}

	if succeeded {
		log.Info("Restore job succeeded. Moving to Completed phase.")
		_, err := updateResticRestoreStatus(ctx, deps, restore, v1.PhaseCompleted, metav1.Now(), job.Name)
		return ctrl.Result{}, err
	}

	// Job failed
	log.Errorw("Restore job failed. Moving to Failed phase.")
	_, err := updateResticRestoreStatus(ctx, deps, restore, v1.PhaseFailed, metav1.Now(), job.Name)
	return ctrl.Result{}, err
}

// updateResticRestoreStatus updates the status of the ResticRestore resource.
func updateResticRestoreStatus(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore, phase v1.Phase, transitionTime metav1.Time, jobName string) (ctrl.Result, error) {
	restore.Status.Phase = phase
	if phase != v1.PhaseFailed {
		restore.Status.Error = "" // Clear previous messages
	}

	if jobName != "" {
		restore.Status.Job.JobRef = &corev1.LocalObjectReference{Name: jobName}
	}
	if restore.Status.CompletionTime == nil && (phase == v1.PhaseCompleted || phase == v1.PhaseFailed) {
		restore.Status.CompletionTime = &transitionTime
	}

	if err := deps.Client.Status().Update(ctx, restore); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update restore status: %w", err)
	}

	if phase == v1.PhaseFailed {
		return ctrl.Result{RequeueAfter: constants.FailedRequeueInterval}, nil
	}
	if phase == v1.PhaseCompleted {
		return ctrl.Result{}, nil
	}

	return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
}
