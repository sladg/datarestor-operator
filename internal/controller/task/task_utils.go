package task_util

import (
	"context"

	"github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RemoveTasksByLabels deletes all Task resources matching the given labels.
func RemoveTasksByConfig(ctx context.Context, deps *utils.Dependencies, config *v1alpha1.Config) error {
	labels := map[string]string{
		constants.LabelTaskParentName:      config.Name,
		constants.LabelTaskParentNamespace: config.Namespace,
	}

	tasks := &v1alpha1.TaskList{}
	err := deps.List(ctx, tasks, client.MatchingLabels(labels))
	if err != nil {
		return err
	}
	for _, task := range tasks.Items {
		if err := deps.Delete(ctx, &task); err != nil {
			return err
		}
	}

	return nil
}
