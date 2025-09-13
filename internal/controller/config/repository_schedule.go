package config_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	task_util "github.com/sladg/datarestor-operator/internal/controller/task"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/restic"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// hasActiveScheduledBackupTask returns true if there is any active (pending/starting/running)
// scheduled backup task for the given PVC under the provided config. Errors are treated as no-active.
func hasActiveScheduledBackupTask(ctx context.Context, deps *utils.Dependencies, config *v1.Config, pvcName string) bool {
	existing := &v1.TaskList{}
	if err := deps.List(ctx, existing, client.MatchingLabels(map[string]string{
		constants.LabelTaskParentName:      config.Name,
		constants.LabelTaskParentNamespace: config.Namespace,
		constants.LabelTaskType:            string(v1.TaskTypeBackupScheduled),
		constants.LabelTaskPVC:             pvcName,
	})); err != nil {
		return false
	}

	for _, t := range existing.Items {
		if t.Status.State == v1.TaskStatePending || t.Status.State == v1.TaskStateStarting || t.Status.State == v1.TaskStateRunning {
			return true
		}
	}
	return false
}

func ScheduleBackupRepositories(ctx context.Context, deps *utils.Dependencies, config *v1.Config, pvcResult ListPVCsForConfigResult) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[ScheduleBackupRepositories]").With(
		"config", config.Name,
		"configNamespace", config.Namespace,
	)

	for i, spec := range config.Spec.Repositories {
		repository := &config.Status.Repositories[i]

		shouldUpdate := utils.ShouldPerformScheduledOperation(spec.BackupSchedule, repository.LastScheduledBackupRun, repository.InitializedAt)
		if !shouldUpdate {
			continue
		}

		// We create tasks first, then update the last scheduled backup run
		// So to avoid race condition, we have to check if there is already a pending/starting/running task for this PVC
		repository.LastScheduledBackupRun = metav1.Now()

		for _, pvc := range pvcResult.MatchedPVCs {
			log := logger.With("pvc", pvc.Name, "pvcNamespace", pvc.Namespace)

			// Avoid duplicate scheduled tasks
			if hasActiveScheduledBackupTask(ctx, deps, config, pvc.Name) {
				log.Infow("Scheduled backup task already active for PVC, skipping duplicate")
				continue
			}

			taskName, taskSpecName := task_util.GenerateUniqueName(task_util.UniqueNameParams{
				PVC:      pvc,
				TaskType: v1.TaskTypeBackupScheduled,
			})

			// Build restic args for this repository and pvc
			params := restic.MakeArgsParams{
				Repositories: []v1.RepositorySpec{spec},
				Env:          config.Spec.Env,
				TargetPVC:    pvc,
				Annotation:   "now",
				TaskName:     taskName,
			}

			args := restic.MakeBackupArgs(params)
			selectedRepo := restic.SelectRepository(params)
			mergedEnv := restic.MergeEnvs(config.Spec.Env, selectedRepo.Env)

			backupTask := task_util.BuildTask(task_util.BuildTaskParams{
				Config:       config,
				PVC:          pvc,
				Env:          mergedEnv,
				Args:         args,
				TaskName:     taskName,
				TaskSpecName: taskSpecName,
				TaskType:     v1.TaskTypeBackupScheduled,
				StopPods:     config.Spec.StopPods,
			})

			if err := deps.Create(ctx, &backupTask); err != nil {
				log.Errorw("Failed to create scheduled backup task for PVC", "error", err)
			} else {
				log.Info("Created scheduled backup task for PVC")
			}
		}

		// We have scheduled lot of backups, let them process before reconcile again
		return true, constants.LongerRequeueInterval, nil
	}

	return false, -1, nil
}

func ScheduleForgetRepositories(ctx context.Context, deps *utils.Dependencies, config *v1.Config) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[ScheduleForgetRepositories]").With(
		"config", config.Name,
		"configNamespace", config.Namespace,
	)

	for i, spec := range config.Spec.Repositories {
		repository := &config.Status.Repositories[i]

		shouldUpdate := utils.ShouldPerformScheduledOperation(spec.ForgotSchedule, repository.LastScheduledForgetRun, repository.InitializedAt)
		if !shouldUpdate {
			continue
		}

		logger.Info("Scheduled forget operation triggered")

		// Update the last scheduled forget run time
		repository.LastScheduledForgetRun = metav1.Now()

		// Execute forget command inline
		selectedRepo := restic.SelectRepository(restic.MakeArgsParams{
			Repositories: []v1.RepositorySpec{spec},
			Env:          config.Spec.Env,
		})
		mergedEnv := restic.MergeEnvs(config.Spec.Env, selectedRepo.Env)

		output, err := restic.ExecForget(ctx, deps.Logger, selectedRepo.Target, mergedEnv, spec.ForgetArgs...)
		if err != nil {
			logger.Errorw("Failed to execute forget operation", "error", err, "output", output)
			return true, constants.DefaultRequeueInterval, err
		}

		logger.Infow("Successfully executed forget operation", "output", output)

		// We have executed forget, let it process before reconcile again
		return true, constants.LongerRequeueInterval, nil
	}

	return false, -1, nil
}
