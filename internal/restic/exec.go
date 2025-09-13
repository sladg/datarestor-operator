package restic

import (
	"bytes"
	"context"
	"encoding/json"
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

// setupResticCommandWithCache creates a restic command with cache environment setup for commands that need --no-cache
func setupResticCommandWithCache(ctx context.Context, args []string, env []corev1.EnvVar) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "restic", args...)
	// Provide sane defaults first, then allow provided env to override
	cmd.Env = append(cmd.Env, "HOME=/tmp", "RESTIC_CACHE_DIR=/tmp/restic-cache")
	cmd.Env = append(cmd.Env, convertEnv(env)...)
	return cmd
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

	cmd := setupResticCommandWithCache(ctx, arguments, env)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorw("Restic check snapshot exists failed", err, "output", string(output))
		return string(output), fmt.Errorf("restic check failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}

// Snapshot is a minimal representation of a restic snapshot returned by --json
type Snapshot struct {
	ID      string   `json:"id"`
	ShortID string   `json:"short_id"`
	Time    string   `json:"time"`
	Tags    []string `json:"tags"`
	Host    string   `json:"hostname"`
}

// ExecListSnapshots executes `restic snapshots --json` with the provided args and parses the JSON output.
func ExecListSnapshots(ctx context.Context, logger *zap.SugaredLogger, env []corev1.EnvVar, args ...string) ([]Snapshot, error) {
	// Use cache directory to avoid HOME/XDG cache warnings; parse stdout only
	arguments := []string{"snapshots", "--json"}
	arguments = append(arguments, args...)

	cmd := setupResticCommandWithCache(ctx, arguments, env)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logger.Errorw("Restic list snapshots failed", err, "stderr", stderr.String(), "stdout", stdout.String())
		return nil, fmt.Errorf("restic snapshots failed: %w, stderr: %s", err, stderr.String())
	}

	out := stdout.Bytes()
	var snaps []Snapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		logger.Errorw("Failed to parse restic snapshots JSON", err, "stdout", stdout.String(), "stderr", stderr.String())
		return nil, fmt.Errorf("parse snapshots json failed: %w", err)
	}
	return snaps, nil
}

// ExecForget executes `restic forget` with the provided args.
func ExecForget(ctx context.Context, logger *zap.SugaredLogger, repoTarget string, env []corev1.EnvVar, args ...string) (string, error) {
	arguments := []string{"forget", "--repo", repoTarget}
	arguments = append(arguments, args...)

	cmd := setupResticCommandWithCache(ctx, arguments, env)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Errorw("Restic forget failed", err, "output", string(output))
		return string(output), fmt.Errorf("restic forget failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
