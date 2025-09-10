package config_util

import (
	"context"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// It syncs Spec into Status
func Init(ctx context.Context, deps *utils.Dependencies, config *v1.Config) (bool, time.Duration, error) {
	logger := deps.Logger.Named("[InitConfig]").With("name", config.Name, "namespace", config.Namespace)

	if !config.Status.InitializedAt.IsZero() {
		return false, -1, nil
	}

	logger.Info("Config not initialized yet, initializing...")
	config.Status.InitializedAt = metav1.Now()

	// Add more stuff if needed here

	return true, constants.ImmediateRequeueInterval, nil
}
