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
	logger := deps.Logger.Named("[PVCBackup]").With(
		"config", config.Name,
		"configNamespace", config.Namespace,
		"pvc", pvc.Name,
		"pvcNamespace", pvc.Namespace,
	)

	if pvc.Annotations[constants.AnnBackup] == "" {
		return false, -1, nil
	}

	logger.Info("Backup annotation detected")

	taskName, taskSpecName := task_util.GenerateUniqueName(task_util.UniqueNameParams{
		PVC:      pvc,
		TaskType: v1.TaskTypeBackupManual,
	})

	params := restic.MakeArgsParams{
		Repositories: config.Spec.Repositories,
		Env:          config.Spec.Env,
		TargetPVC:    pvc,
		Annotation:   pvc.Annotations[constants.AnnBackup],
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
		TaskType:     v1.TaskTypeBackupManual,
		TaskName:     taskName,
		TaskSpecName: taskSpecName,
		StopPods:     config.Spec.StopPods,
	})

	if err := deps.Create(ctx, &backupTask); err != nil {
		logger.Errorw("Failed to create backup task for PVC", "error", err)
	} else {
		logger.Info("Created backup task")
	}

	// Remove annotation to avoid loops
	annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnBackup: ""})
	pvc.SetAnnotations(annotations)

	if err := deps.Update(ctx, pvc); err != nil {
		logger.Errorw("Failed to clear backup annotation on PVC", "error", err)
	} else {
		logger.Info("Cleared backup annotation")
	}

	return true, constants.LongerRequeueInterval, nil
}

func PVCRestore(ctx context.Context, deps *utils.Dependencies, config *v1.Config, pvc *corev1.PersistentVolumeClaim) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[PVCRestore]").With(
		"config", config.Name,
		"configNamespace", config.Namespace,
		"pvc", pvc.Name,
		"pvcNamespace", pvc.Namespace,
	)

	if pvc.Annotations[constants.AnnRestore] == "" {
		return false, -1, nil
	}

	// Skip PVCs that are being deleted to prevent race conditions
	if utils.IsPVCBeingDeleted(pvc) {
		logger.Info("Skipping restore for PVC in terminating state")
		return false, -1, nil
	}

	logger.Info("Restore annotation detected")

	taskName, taskSpecName := task_util.GenerateUniqueName(task_util.UniqueNameParams{
		PVC:      pvc,
		TaskType: v1.TaskTypeRestoreManual,
	})

	params := restic.MakeArgsParams{
		Repositories: config.Spec.Repositories,
		Env:          config.Spec.Env,
		TargetPVC:    pvc,
		Annotation:   pvc.Annotations[constants.AnnRestore],
		TaskName:     taskName,
	}
	args := restic.MakeRestoreArgs(params)
	selectedRepo := restic.SelectRepository(params)
	mergedEnv := restic.MergeEnvs(config.Spec.Env, selectedRepo.Env)

	restoreTask := task_util.BuildTask(task_util.BuildTaskParams{
		Config:       config,
		PVC:          pvc,
		Env:          mergedEnv,
		Args:         args,
		TaskName:     taskName,
		TaskSpecName: taskSpecName,
		TaskType:     v1.TaskTypeRestoreManual,
		StopPods:     config.Spec.StopPods,
	})

	if err := deps.Create(ctx, &restoreTask); err != nil {
		logger.Errorw("Failed to create restore task for PVC", "error", err)
	} else {
		logger.Info("Created restore task")
	}

	annotations := utils.MakeAnnotation(pvc.Annotations, map[string]string{constants.AnnRestore: ""})
	pvc.SetAnnotations(annotations)
	if err := deps.Update(ctx, pvc); err != nil {
		logger.Errorw("Failed to clear restore annotation on PVC", "error", err)
	} else {
		logger.Info("Cleared restore annotation")
	}

	return true, constants.LongerRequeueInterval, nil
}
