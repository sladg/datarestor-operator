package watches

import (
	"context"

	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func RequestTasksForConfig(deps *utils.Dependencies) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		configName, ok := obj.GetLabels()[constants.LabelTaskParentName]
		if !ok {
			return nil
		}
		configNamespace, ok := obj.GetLabels()[constants.LabelTaskParentNamespace]
		if !ok {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Name:      configName,
					Namespace: configNamespace,
				},
			},
		}
	}
}
