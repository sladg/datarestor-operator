package watches

import (
	"context"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// For each selector, find a PVCs that match and return a request for each PVC
func RequestPVCsForConfig(deps *utils.Dependencies) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		config, ok := obj.(*v1.Config)
		if !ok {
			return nil
		}

		requests := []types.NamespacedName{}

		for _, selector := range config.Spec.Selectors {
			opts := []client.ListOption{}
			if selector.MatchLabels != nil {
				opts = append(opts, client.MatchingLabels(selector.MatchLabels))
			}
			if selector.MatchNamespaces != nil {
				for _, namespace := range selector.MatchNamespaces {
					opts = append(opts, client.InNamespace(namespace))
				}
			}
			pvcs := &corev1.PersistentVolumeClaimList{}
			err := deps.List(ctx, pvcs, opts...)
			if err != nil {
				return nil
			}
			for _, pvc := range pvcs.Items {
				requests = append(requests, types.NamespacedName{
					Name:      pvc.Name,
					Namespace: pvc.Namespace,
				})
			}
		}

		seen := make(map[types.NamespacedName]bool)
		result := []reconcile.Request{}

		for _, name := range requests {
			if !seen[name] {
				seen[name] = true
				result = append(result, reconcile.Request{
					NamespacedName: name,
				})
			}
		}

		return result
	}
}
