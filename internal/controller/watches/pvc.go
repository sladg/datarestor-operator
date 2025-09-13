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

// For each PVC event, find matching Configs and return reconciliation requests
func RequestPVCsForConfig(deps *utils.Dependencies) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		pvc, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			return nil
		}

		// Get all Configs to check if this PVC matches any selectors
		configs := &v1.ConfigList{}
		err := deps.List(ctx, configs)
		if err != nil {
			return nil
		}

		var requests []reconcile.Request

		// Check if this PVC matches any Config's selectors
		for _, config := range configs.Items {
			if matchesConfigSelectors(pvc, &config) {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      config.Name,
						Namespace: config.Namespace,
					},
				})
			}
		}

		return requests
	}
}

// matchesConfigSelectors checks if a PVC matches any of the Config's selectors
func matchesConfigSelectors(pvc *corev1.PersistentVolumeClaim, config *v1.Config) bool {
	for _, selector := range config.Spec.Selectors {
		// Check namespace match
		if selector.MatchNamespaces != nil {
			namespaceMatch := false
			for _, namespace := range selector.MatchNamespaces {
				if pvc.Namespace == namespace {
					namespaceMatch = true
					break
				}
			}
			if !namespaceMatch {
				continue
			}
		}

		// Check label match
		if selector.MatchLabels != nil {
			labelMatch := true
			for key, value := range selector.MatchLabels {
				if pvc.Labels[key] != value {
					labelMatch = false
					break
				}
			}
			if !labelMatch {
				continue
			}
		}

		// If we reach here, this selector matches
		return true
	}

	return false
}
