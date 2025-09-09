package restic

import (
	"context"
	"fmt"
	"os/exec"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

// MergeEnvs merges multiple slices of environment variables.
// Later slices take precedence over earlier ones for duplicate keys.
// This allows repository-specific env vars to override global config env vars.
func MergeEnvs(envSlices ...[]corev1.EnvVar) []corev1.EnvVar {
	if len(envSlices) == 0 {
		return nil
	}

	// Use a map to track merged env vars by name
	envMap := make(map[string]corev1.EnvVar)

	// Process each slice in order, later ones override earlier ones
	for _, envSlice := range envSlices {
		for _, envVar := range envSlice {
			envMap[envVar.Name] = envVar
		}
	}

	// Convert map back to slice
	merged := make([]corev1.EnvVar, 0, len(envMap))
	for _, envVar := range envMap {
		merged = append(merged, envVar)
	}

	return merged
}

func convertEnv(env []corev1.EnvVar) []string {
	// Convert envs to kv
	envs := []string{}

	for _, envVar := range env {
		envs = append(envs, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	return envs
}

func ExecInit(ctx context.Context, logger *zap.SugaredLogger, repoTarget string, env []corev1.EnvVar) (string, error) {
	arguments := []string{"init", "--repo", repoTarget}
	// append extras

	cmd := exec.CommandContext(ctx, "restic", arguments...)
	cmd.Env = append(cmd.Env, convertEnv(env)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorw("Restic init failed", err, "output", string(output))
		return string(output), fmt.Errorf("restic init failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

func ExecCheck(ctx context.Context, logger *zap.SugaredLogger, repoTarget string, env []corev1.EnvVar) (string, error) {
	arguments := []string{"check", "--repo", repoTarget}
	// append extras

	cmd := exec.CommandContext(ctx, "restic", arguments...)
	cmd.Env = append(cmd.Env, convertEnv(env)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorw("Restic check failed", err, "output", string(output))
		return string(output), fmt.Errorf("restic check failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

func ExecCheckSnapshotExists(ctx context.Context, logger *zap.SugaredLogger, env []corev1.EnvVar, args ...string) (string, error) {
	arguments := []string{"snapshots"}
	arguments = append(arguments, args...)

	cmd := exec.CommandContext(ctx, "restic", arguments...)
	cmd.Env = append(cmd.Env, convertEnv(env)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorw("Restic check snapshot exists failed", err, "output", string(output))
		return string(output), fmt.Errorf("restic check failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
