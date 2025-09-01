package logic

import (
	"context"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HandleRestoreDeletion handles the deletion of a ResticRestore resource.
// It uses a pipeline of steps to ensure a clean and orderly deletion process.
func HandleRestoreDeletion(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("restore-deletion-pipeline")
	state := constants.DeletionState(restore.Annotations[constants.AnnotationDeletionState])

	// Define the pipeline steps
	steps := []utils.Step{
		{
			Condition: func() bool { return state == "" },
			Action: func() (ctrl.Result, error) {
				return initializeDeletionState(ctx, deps, restore)
			},
		},
		{
			Condition: func() bool { return state == constants.DeletionStateCleanup },
			Action: func() (ctrl.Result, error) {
				return cleanupRestoreResources(ctx, deps, restore)
			},
		},
		{
			Condition: func() bool { return state == constants.DeletionStateRemoveFinalizer },
			Action: func() (ctrl.Result, error) {
				return removeRestoreFinalizer(ctx, deps, restore)
			},
		},
	}

	// Process the pipeline
	result, err := utils.ProcessSteps(steps...)
	if err != nil {
		log.Errorw("Error processing deletion pipeline", "error", err)
	}
	return result, err
}

// initializeDeletionState sets the initial state for the deletion process.
func initializeDeletionState(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("initialize-deletion-state")
	if err := utils.AnnotateResources(ctx, deps, []client.Object{restore}, map[string]string{
		constants.AnnotationDeletionState: string(constants.DeletionStateCleanup),
	}); err != nil {
		log.Errorw("Failed to set initial deletion state", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}
	return ctrl.Result{Requeue: true}, nil
}

// cleanupRestoreResources performs the actual cleanup of resources associated with the restore.
func cleanupRestoreResources(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("cleanup-restore-resources")

	// Clean up any restore jobs
	if err := cleanupRestoreJobs(ctx, deps, restore); err != nil {
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	// If this was an automated restore, remove workload finalizers
	if restore.Spec.Type == backupv1alpha1.RestoreTypeAutomated {
		if err := handleAutomatedRestoreFinalizers(ctx, deps, restore); err != nil {
			return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
		}
	}

	// Transition to the next state
	if err := utils.AnnotateResources(ctx, deps, []client.Object{restore}, map[string]string{
		constants.AnnotationDeletionState: string(constants.DeletionStateRemoveFinalizer),
	}); err != nil {
		log.Errorw("Failed to update deletion state", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

// cleanupRestoreJobs finds and deletes any Kubernetes Jobs associated with the restore.
func cleanupRestoreJobs(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) error {
	log := deps.Logger.Named("cleanup-restore-jobs")
	if restore.Status.Job.JobRef == nil || restore.Status.Job.JobRef.Name == "" {
		return nil
	}

	jobSelector := backupv1alpha1.Selector{
		Namespaces: []string{restore.Namespace},
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				"app.kubernetes.io/instance":   restore.Name,
				"app.kubernetes.io/managed-by": "autorestore-backup-operator",
				"job-type":                     "restore",
			},
		},
	}
	jobList := &batchv1.JobList{}
	jobs, err := utils.FindMatchingResources[*batchv1.Job](ctx, deps, []backupv1alpha1.Selector{jobSelector}, jobList)
	if err != nil {
		log.Errorw("Failed to find restore jobs", "error", err)
		return err
	}

	for _, job := range jobs {
		if err := deps.Client.Delete(ctx, job); err != nil {
			log.Errorw("Failed to delete restore job", "job", job.Name, "error", err)
			return err
		}
	}
	return nil
}

// handleAutomatedRestoreFinalizers removes finalizers from workloads associated with an automated restore.
func handleAutomatedRestoreFinalizers(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) error {
	log := deps.Logger.Named("handle-automated-restore-finalizers")
	pvcSelector := utils.SelectorForResource(restore.Spec.TargetPVC.Name, restore.Spec.TargetPVC.Namespace)
	pvcs, err := utils.FindMatchingPVCs(ctx, deps, &pvcSelector)
	if err != nil {
		log.Errorw("Failed to find PVC", "error", err)
		return err
	}

	if len(pvcs) > 0 {
		if err := removeWorkloadFinalizers(ctx, deps, &pvcs[0]); err != nil {
			log.Errorw("Failed to remove workload finalizers", "error", err)
			return err
		}
	}
	return nil
}

// removeRestoreFinalizer removes the finalizer from the ResticRestore resource.
func removeRestoreFinalizer(ctx context.Context, deps *utils.Dependencies, restore *backupv1alpha1.ResticRestore) (ctrl.Result, error) {
	log := deps.Logger.Named("remove-restore-finalizer")
	if err := utils.RemoveFinalizer(ctx, deps, restore, constants.ResticRestoreFinalizer); err != nil {
		log.Errorw("Failed to remove finalizer", "error", err)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, err
	}
	return ctrl.Result{}, nil
}

func removeWorkloadFinalizers(ctx context.Context, deps *utils.Dependencies, pvc *corev1.PersistentVolumeClaim) error {
	workloads, err := utils.FindWorkloadsForPVC(ctx, deps, pvc.Name, pvc.Namespace)
	if err != nil {
		return err
	}

	for _, workload := range workloads {
		if err := utils.RemoveFinalizer(ctx, deps, workload, constants.WorkloadFinalizer); err != nil {
			return err
		}
	}

	return nil
}
