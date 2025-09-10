package config_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	task_util "github.com/sladg/datarestor-operator/internal/controller/task"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/restic"
	corev1 "k8s.io/api/core/v1"
)

func PVCBackup(ctx context.Context, deps *utils.Dependencies, config *v1.Config, pvc *corev1.PersistentVolumeClaim) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[PVCBackup]").With("name", config.Name, "namespace", config.Namespace)

	if pvc.Annotations[constants.AnnBackup] == "" {
		return false, 0, nil
	}

	logger.Infow("Backup annotation detected on PVC", "pvc", pvc.Name)

	params := restic.MakeArgsParams{
		Repositories: config.Spec.Repositories,
		Env:          config.Spec.Env,
		TargetPVC:    pvc,
		Annotation:   pvc.Annotations[constants.AnnBackup],
	}
	args := restic.MakeBackupArgs(params)
	selectedRepo := restic.SelectRepository(params)
	mergedEnv := restic.MergeEnvs(config.Spec.Env, selectedRepo.Env)

	backupTask := task_util.BuildTask(task_util.BuildTaskParams{
		Config:   config,
		PVC:      pvc,
		Env:      mergedEnv,
		Args:     args,
		TaskType: v1.TaskTypeBackupManual,
		StopPods: config.Spec.StopPods,
	})

	if err := deps.Create(ctx, &backupTask); err != nil {
		logger.Errorw("Failed to create backup task for PVC", "pvc", pvc.Name, "error", err)
	} else {
		logger.Infow("Created backup task for PVC", "pvc", pvc.Name)
	}

	// Remove annotation to avoid loops
	annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnBackup: ""})
	pvc.SetAnnotations(annotations)

	if err := deps.Update(ctx, pvc); err != nil {
		logger.Errorw("Failed to clear backup annotation on PVC", "pvc", pvc.Name, "error", err)
	} else {
		logger.Infow("Cleared backup annotation on PVC", "pvc", pvc.Name)
	}

	return true, constants.LongerRequeueInterval, nil
}

func PVCRestore(ctx context.Context, deps *utils.Dependencies, config *v1.Config, pvc *corev1.PersistentVolumeClaim) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[PVCRestore]").With("name", config.Name, "namespace", config.Namespace)

	if pvc.Annotations[constants.AnnRestore] == "" {
		return false, 0, nil
	}

	logger.Infow("Restore annotation detected on PVC", "pvc", pvc.Name)

	params := restic.MakeArgsParams{
		Repositories: config.Spec.Repositories,
		Env:          config.Spec.Env,
		TargetPVC:    pvc,
		Annotation:   pvc.Annotations[constants.AnnRestore],
	}
	args := restic.MakeRestoreArgs(params)
	selectedRepo := restic.SelectRepository(params)
	mergedEnv := restic.MergeEnvs(config.Spec.Env, selectedRepo.Env)

	restoreTask := task_util.BuildTask(task_util.BuildTaskParams{
		Config:   config,
		PVC:      pvc,
		Env:      mergedEnv,
		Args:     args,
		TaskType: v1.TaskTypeRestoreManual,
		StopPods: config.Spec.StopPods,
	})

	if err := deps.Create(ctx, &restoreTask); err != nil {
		logger.Errorw("Failed to create restore task for PVC", "pvc", pvc.Name, "error", err)
	} else {
		logger.Infow("Created restore task for PVC", "pvc", pvc.Name)
	}

	annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnRestore: ""})
	pvc.SetAnnotations(annotations)
	if err := deps.Update(ctx, pvc); err != nil {
		logger.Errorw("Failed to clear restore annotation on PVC", "pvc", pvc.Name, "error", err)
	} else {
		logger.Infow("Cleared restore annotation on PVC", "pvc", pvc.Name)
	}

	return true, constants.LongerRequeueInterval, nil
}
