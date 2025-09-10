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
	logger := deps.Logger.Named("[AutoRestore]").With(
		"name", config.Name,
		"namespace", config.Namespace,
	)

	if len(pvcResult.UnclaimedPVCs) == 0 {
		return false, -1, nil
	}

	if !config.Spec.AutoRestore {
		return false, -1, nil
	}

	for _, pvc := range pvcResult.UnclaimedPVCs {
		log := logger.With("pvc", pvc.Name, "pvcNamespace", pvc.Namespace)
		log.Info("New unclaimed PVC detected")

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
			log.Errorw("Failed to create restore task for new PVC", "error", err)
			return true, constants.QuickRequeueInterval, nil
		}

		log.Info("Created restore task for new PVC")

		// Mark PVC for auto-restore
		annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnAutoRestored: "true"})
		pvc.SetAnnotations(annotations)

		if err := deps.Update(ctx, pvc); err != nil {
			log.Errorw("Failed to mark PVC as auto-restored", "error", err)
			return true, constants.QuickRequeueInterval, err
		}

		log.Info("Marked PVC as auto-restored")
	}

	return true, constants.DefaultRequeueInterval, nil
}
