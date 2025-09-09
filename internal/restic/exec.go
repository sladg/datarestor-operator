package restic

import (
	"context"
	"fmt"
	"os/exec"

	corev1 "k8s.io/api/core/v1"
)

func convertEnv(env []corev1.EnvVar) []string {
	// Convert envs to kv
	envs := []string{}

	for _, envVar := range env {
		envs = append(envs, fmt.Sprintf("%s=%s", envVar.Name, envVar.Value))
	}

	return envs
}

func ExecInit(ctx context.Context, repoTarget string, env []corev1.EnvVar) (string, error) {
	arguments := []string{"init", "--repo", repoTarget}
	// append extras

	cmd := exec.CommandContext(ctx, "restic", arguments...)
	cmd.Env = append(cmd.Env, convertEnv(env)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("restic init failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

func ExecCheck(ctx context.Context, repoTarget string, env []corev1.EnvVar) (string, error) {
	arguments := []string{"check", "--repo", repoTarget}
	// append extras

	cmd := exec.CommandContext(ctx, "restic", arguments...)
	cmd.Env = append(cmd.Env, convertEnv(env)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("restic check failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

func ExecCheckSnapshotExists(ctx context.Context, env []corev1.EnvVar, args ...string) (string, error) {
	arguments := []string{"snapshots"}
	arguments = append(arguments, args...)

	cmd := exec.CommandContext(ctx, "restic", arguments...)
	cmd.Env = append(cmd.Env, convertEnv(env)...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return string(output), fmt.Errorf("restic check failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
