package utils

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sladg/datarestor-operator/internal/constants"
	appsv1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// workloadInfo is a temporary struct used for (un)marshalling replica counts.
type workloadInfo struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	Replicas int32  `json:"replicas"`
}

func GetOriginalReplicasInfo(ctx context.Context, deps *Dependencies, workloads []client.Object) (map[string]workloadInfo, error) {
	originalReplicas := make(map[string]workloadInfo)
	for _, workload := range workloads {
		switch w := workload.(type) {
		case *appsv1.Deployment:
			originalReplicas[string(w.UID)] = workloadInfo{Kind: "Deployment", Name: w.Name, Replicas: *w.Spec.Replicas}
		case *appsv1.StatefulSet:
			originalReplicas[string(w.UID)] = workloadInfo{Kind: "StatefulSet", Name: w.Name, Replicas: *w.Spec.Replicas}
		}
	}

	return originalReplicas, nil
}

func SetOriginalReplicasAnnotation(ctx context.Context, deps *Dependencies, owner client.Object, originalReplicas map[string]workloadInfo) error {
	jsonData, err := json.Marshal(originalReplicas)
	if err != nil {
		return fmt.Errorf("failed to marshal original replica counts: %w", err)
	}

	annotations := owner.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[constants.AnnotationOriginalReplicas] = string(jsonData)
	owner.SetAnnotations(annotations)

	return deps.Update(ctx, owner)
}

func LoadOriginalReplicasFromAnnotation(owner client.Object) (map[string]workloadInfo, error) {
	annotations := owner.GetAnnotations()
	jsonData, ok := annotations[constants.AnnotationOriginalReplicas]
	if !ok || jsonData == "" {
		return nil, nil // No annotation found, not an error.
	}

	originalReplicas := map[string]workloadInfo{}
	if err := json.Unmarshal([]byte(jsonData), &originalReplicas); err != nil {
		return nil, fmt.Errorf("failed to unmarshal original replica counts: %w", err)
	}
	return originalReplicas, nil
}

// RemoveOriginalReplicasAnnotation removes the original replica count annotation from the owner.
func RemoveOriginalReplicasAnnotation(ctx context.Context, deps *Dependencies, owner client.Object) error {
	annotations := owner.GetAnnotations()
	if _, ok := annotations[constants.AnnotationOriginalReplicas]; !ok {
		return nil // Annotation doesn't exist, nothing to do.
	}

	delete(annotations, constants.AnnotationOriginalReplicas)
	owner.SetAnnotations(annotations)
	return deps.Update(ctx, owner)
}

// AnnotateResources applies a set of annotations to a list of resources.
// If a value in the annotations map is nil, the corresponding annotation is removed.
func AnnotateResources(ctx context.Context, deps *Dependencies, resources []client.Object, annotations map[string]*string) error {
	for _, resource := range resources {
		currentAnnotations := resource.GetAnnotations()
		if currentAnnotations == nil {
			currentAnnotations = make(map[string]string)
		}
		modified := false
		for key, value := range annotations {
			if value == nil {
				// A nil value indicates the annotation should be removed.
				if _, exists := currentAnnotations[key]; exists {
					delete(currentAnnotations, key)
					modified = true
				}
			} else {
				if currentAnnotations[key] != *value {
					currentAnnotations[key] = *value
					modified = true
				}
			}
		}

		if modified {
			resource.SetAnnotations(currentAnnotations)
			if err := deps.Update(ctx, resource); err != nil {
				return fmt.Errorf("failed to update annotations for %s %s: %w", resource.GetObjectKind().GroupVersionKind().Kind, resource.GetName(), err)
			}
		}
	}
	return nil
}
