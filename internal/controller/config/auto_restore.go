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

// Process new unclaimed PVCs for auto-restore. This has priority over other operations.
//
// TODO: Allow for repository-level auto-restore override
func AutoRestore(ctx context.Context, deps *utils.Dependencies, config *v1.Config, pvcResult ListPVCsForConfigResult) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[AutoRestore]").With("name", config.Name, "namespace", config.Namespace)

	if len(pvcResult.UnclaimedPVCs) == 0 {
		return false, 0, nil
	}

	if !config.Spec.AutoRestore {
		return false, 0, nil
	}

	for _, pvc := range pvcResult.UnclaimedPVCs {
		logger.Infow("New unclaimed PVC detected", "pvc", pvc.Name)

		params := restic.MakeArgsParams{
			Repositories: config.Spec.Repositories,
			Env:          config.Spec.Env,
			TargetPVC:    pvc,
			Annotation:   "", // Let Task figure out best restore point
		}

		args := restic.MakeRestoreArgs(params)
		selectedRepo := restic.SelectRepository(params)
		mergedEnv := restic.MergeEnvs(config.Spec.Env, selectedRepo.Env)

		// Create restore task for each PVC
		restoreTask := task_util.BuildTask(task_util.BuildTaskParams{
			Config:   config,
			PVC:      pvc,
			Env:      mergedEnv,
			Args:     args,
			TaskType: v1.TaskTypeRestoreAutomated,
			StopPods: config.Spec.StopPods,
		})

		if err := deps.Create(ctx, &restoreTask); err != nil {
			logger.Errorw("Failed to create restore task for new PVC", "pvc", pvc.Name, "error", err)
			return true, constants.QuickRequeueInterval, nil
		}

		logger.Infow("Created restore task for new PVC", "pvc", pvc.Name)

		// Mark PVC for auto-restore
		annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnAutoRestored: "true"})
		pvc.SetAnnotations(annotations)

		if err := deps.Update(ctx, pvc); err != nil {
			logger.Errorw("Failed to mark PVC as auto-restored", "pvc", pvc.Name, "error", err)
			return true, constants.QuickRequeueInterval, err
		}

		logger.Infow("Marked PVC as auto-restored", "pvc", pvc.Name)
	}

	return true, constants.DefaultRequeueInterval, nil
}
