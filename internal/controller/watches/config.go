package watches

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func RequestConfigs(deps *utils.Dependencies) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		configs := &v1.ConfigList{}
		err := deps.List(ctx, configs)
		if err != nil {
			return nil
		}

		var requests []reconcile.Request
		for _, config := range configs.Items {
			requests = append(requests, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Name:      config.Name,
					Namespace: config.Namespace,
				},
			})
		}

		return requests
	}
}
