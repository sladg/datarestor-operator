package config_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	task_util "github.com/sladg/datarestor-operator/internal/controller/task"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/restic"
)

func ConfigBackup(ctx context.Context, deps *utils.Dependencies, config *v1.Config, pvcResult ListPVCsForConfigResult) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[ConfigBackup]").With(
		"config", config.Name,
		"configNamespace", config.Namespace,
	)

	if config.Annotations[constants.AnnBackup] == "" {
		return false, -1, nil
	}

	logger.Info("Force backup annotation detected, removing ...")

	for _, pvc := range pvcResult.MatchedPVCs {
		log := logger.With("pvc", pvc.Name, "pvcNamespace", pvc.Namespace)

		// Prepare backup args
		params := restic.MakeArgsParams{
			Repositories: config.Spec.Repositories,
			Env:          config.Spec.Env,
			TargetPVC:    pvc,
			Annotation:   config.Annotations[constants.AnnBackup],
		}

		args := restic.MakeBackupArgs(params)
		selectedRepo := restic.SelectRepository(params)
		mergedEnv := restic.MergeEnvs(config.Spec.Env, selectedRepo.Env)

		// Prepare backup task for pvc
		backupTask := task_util.BuildTask(task_util.BuildTaskParams{
			Config:   config,
			PVC:      pvc,
			Env:      mergedEnv,
			Args:     args,
			TaskType: v1.TaskTypeBackupManual,
			StopPods: config.Spec.StopPods,
		})

		// Create it
		if err := deps.Create(ctx, &backupTask); err != nil {
			log.Errorw("Failed to create backup task for PVC", "error", err)
		} else {
			log.Info("Created backup task")
		}
	}

	// Remove annotation to avoid loops
	annotations := utils.MakeAnnotation(config.Annotations, map[string]string{constants.AnnBackup: ""})
	config.SetAnnotations(annotations)

	return true, constants.LongerRequeueInterval, nil
}
