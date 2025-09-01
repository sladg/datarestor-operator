package utils

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// annotateResources updates annotations on k8s resources.
// Empty string value removes the annotation.
func AnnotateResources(ctx context.Context, deps *Dependencies, resources []client.Object, annotations map[string]string) error {
	log := deps.Logger.Named("annotate-resources")

	for i := range resources {
		resource := resources[i]
		currentAnnotations := resource.GetAnnotations()
		if currentAnnotations == nil {
			currentAnnotations = make(map[string]string)
		}

		modified := false
		for key, value := range annotations {
			if value == "" {
				// Remove annotation if it exists
				if _, exists := currentAnnotations[key]; exists {
					delete(currentAnnotations, key)
					modified = true
				}
			} else {
				// Add or update annotation if value differs
				if currentValue, exists := currentAnnotations[key]; !exists || currentValue != value {
					currentAnnotations[key] = value
					modified = true
				}
			}
		}

		if modified {
			resource.SetAnnotations(currentAnnotations)
			if err := deps.Client.Update(ctx, resource); err != nil {
				log.Errorw("Failed to update resource annotations",
					"kind", resource.GetObjectKind().GroupVersionKind().Kind,
					"name", resource.GetName(),
					"namespace", resource.GetNamespace(),
					"error", err)
				return fmt.Errorf("failed to update annotations on resource %s: %w", resource.GetName(), err)
			}
			log.Infow("Successfully updated resource annotations",
				"kind", resource.GetObjectKind().GroupVersionKind().Kind,
				"name", resource.GetName(),
				"namespace", resource.GetNamespace())
		}
	}

	return nil
}
