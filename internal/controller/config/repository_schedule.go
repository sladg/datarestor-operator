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
)

func ScheduleBackupRepositories(ctx context.Context, deps *utils.Dependencies, config *v1.Config, pvcResult ListPVCsForConfigResult) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[ScheduleBackupRepositories]").With("name", config.Name, "namespace", config.Namespace)

	for i, spec := range config.Spec.Repositories {
		repository := &config.Status.Repositories[i]

		shouldUpdate := utils.ShouldPerformBackupFromRepository(spec.BackupSchedule, repository.LastScheduledBackupRun, repository.InitializedAt)
		if !shouldUpdate {
			continue
		}

		repository.LastScheduledBackupRun = metav1.Now()

		for _, pvc := range pvcResult.MatchedPVCs {
			// Build restic args for this repository and pvc
			params := restic.MakeArgsParams{
				Repositories: []v1.RepositorySpec{spec},
				Env:          config.Spec.Env,
				TargetPVC:    pvc,
				Annotation:   "now",
			}

			args := restic.MakeBackupArgs(params)
			selectedRepo := restic.SelectRepository(params)
			mergedEnv := restic.MergeEnvs(config.Spec.Env, selectedRepo.Env)

			backupTask := task_util.BuildTask(task_util.BuildTaskParams{
				Config:   config,
				PVC:      pvc,
				Env:      mergedEnv,
				Args:     args,
				TaskType: v1.TaskTypeBackupScheduled,
				StopPods: config.Spec.StopPods,
			})

			if err := deps.Create(ctx, &backupTask); err != nil {
				logger.Errorw("Failed to create scheduled backup task for PVC", "pvc", pvc.Name, "error", err)
			} else {
				logger.Infow("Created scheduled backup task for PVC", "pvc", pvc.Name)
			}
		}

		// We have scheduled lot of backups, let them process before reconcile again
		return true, constants.LongerRequeueInterval, nil
	}

	return false, 0, nil
}
