package utils

import (
	"context"
	"fmt"
	"reflect"

	"github.com/sladg/datarestor-operator/api/v1alpha1"
	"go.uber.org/zap"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FindMatchingResources discovers k8s resources that match any of the provided selectors
func FindMatchingResources[T client.Object](
	ctx context.Context,
	deps *Dependencies,
	selectors []v1alpha1.Selector,
	list client.ObjectList,
) ([]T, error) {
	log := deps.Logger.Named("resource-finder")
	var matchedResources []T

	// Process each selector
	for _, selector := range selectors {
		// Get resources from specified namespaces
		if len(selector.Namespaces) > 0 {
			for _, ns := range selector.Namespaces {
				if err := deps.List(ctx, list, client.InNamespace(ns)); err != nil {
					log.Debugw("Failed to list resources in namespace", "error", err, "namespace", ns)
					return nil, err
				}
				matchedResources = append(matchedResources, filterMatchingResources[T](list, selector, log)...)
			}
		} else {
			// If no namespaces specified, search all namespaces
			if err := deps.List(ctx, list); err != nil {
				log.Debugw("Failed to list resources in all namespaces", "error", err)
				return nil, err
			}
			matchedResources = append(matchedResources, filterMatchingResources[T](list, selector, log)...)
		}
	}

	log.Debugw("Found matching resources", "count", len(matchedResources))
	return matchedResources, nil
}

// filterMatchingResources filters resources that match the selector
func filterMatchingResources[T client.Object](list client.ObjectList, selector v1alpha1.Selector, log *zap.SugaredLogger) []T {
	var matches []T
	items := reflect.ValueOf(list).Elem().FieldByName("Items")

	for i := 0; i < items.Len(); i++ {
		item := items.Index(i).Interface()

		// Check if T is a pointer type
		var obj T
		if reflect.TypeOf(obj).Kind() == reflect.Ptr {
			// T is a pointer type, so we need to take the address of the item
			itemValue := reflect.ValueOf(item)
			itemPtr := reflect.New(itemValue.Type())
			itemPtr.Elem().Set(itemValue)
			obj = itemPtr.Interface().(T)
		} else {
			// T is a value type, so we can cast directly
			var ok bool
			obj, ok = item.(T)
			if !ok {
				log.Debugw("Failed to cast item to requested type", "item", item)
				continue
			}
		}

		if matchesSelector(obj, &selector) {
			matches = append(matches, obj)
		}
	}
	return matches
}

// FindBackupsByRepository finds all ResticBackup resources for a given ResticRepository
func FindBackupsByRepository(ctx context.Context, deps *Dependencies, namespace, repoName string) ([]v1alpha1.ResticBackup, error) {
	selector := v1alpha1.Selector{
		Namespaces:     []string{namespace},
		RepositoryName: repoName,
	}
	backupList := &v1alpha1.ResticBackupList{}
	backups, err := FindMatchingResources[*v1alpha1.ResticBackup](ctx, deps, []v1alpha1.Selector{selector}, backupList)
	if err != nil {
		return nil, err
	}

	result := make([]v1alpha1.ResticBackup, len(backups))
	for i, backup := range backups {
		result[i] = *backup
	}
	return result, nil
}

// FindRestoresByRepository finds all ResticRestore resources for a given ResticRepository
func FindRestoresByRepository(ctx context.Context, deps *Dependencies, namespace, repoName string) ([]v1alpha1.ResticRestore, error) {
	selector := v1alpha1.Selector{
		Namespaces:     []string{namespace},
		RepositoryName: repoName,
	}
	restoreList := &v1alpha1.ResticRestoreList{}
	restores, err := FindMatchingResources[*v1alpha1.ResticRestore](ctx, deps, []v1alpha1.Selector{selector}, restoreList)
	if err != nil {
		return nil, err
	}

	result := make([]v1alpha1.ResticRestore, len(restores))
	for i, restore := range restores {
		result[i] = *restore
	}
	return result, nil
}

// FindRestoresByBackupName finds all ResticRestore resources for a given ResticBackup name
func FindRestoresByBackupName(ctx context.Context, deps *Dependencies, namespace, backupName string) ([]v1alpha1.ResticRestore, error) {
	var restoreList v1alpha1.ResticRestoreList
	if err := deps.List(ctx, &restoreList, client.InNamespace(namespace)); err != nil {
		return nil, err
	}

	var matchingRestores []v1alpha1.ResticRestore
	for _, restore := range restoreList.Items {
		if restore.Spec.Name == backupName {
			matchingRestores = append(matchingRestores, restore)
		}
	}
	return matchingRestores, nil
}

// FindPodsForJob finds all pods for a given Job
func FindPodsForJob(ctx context.Context, deps *Dependencies, job *batchv1.Job) (*corev1.PodList, error) {
	podList := &corev1.PodList{}
	err := deps.List(ctx, podList, client.InNamespace(job.Namespace), client.MatchingLabels{
		"controller-uid": string(job.UID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for job %s in namespace %s: %w", job.Name, job.Namespace, err)
	}
	return podList, nil
}

// MatchesAnySelector checks if the resource matches any of the selectors in the list.
func MatchesAnySelector(obj client.Object, selectors []v1alpha1.Selector) bool {
	for _, selector := range selectors {
		if matchesSelector(obj, &selector) {
			return true
		}
	}
	return false
}

// FindMatchingSelectors returns all selectors that match the given object.
func FindMatchingSelectors(obj client.Object, selectors []v1alpha1.Selector) []v1alpha1.Selector {
	var matchingSelectors []v1alpha1.Selector
	for _, s := range selectors {
		if matchesSelector(obj, &s) {
			matchingSelectors = append(matchingSelectors, s)
		}
	}
	return matchingSelectors
}

func matchesSelector(obj client.Object, selector *v1alpha1.Selector) bool {
	// Check label selector
	if selector.LabelSelector.MatchLabels != nil || selector.LabelSelector.MatchExpressions != nil {
		labelSelector, err := metav1.LabelSelectorAsSelector(&selector.LabelSelector)
		if err != nil {
			return false
		}
		if !labelSelector.Matches(labels.Set(obj.GetLabels())) {
			return false
		}
	}

	// Check annotation selector
	if len(selector.AnnotationSelector) > 0 {
		for key, value := range selector.AnnotationSelector {
			if obj.GetAnnotations()[key] != value {
				return false
			}
		}
	}

	// Check namespace selector
	if len(selector.Namespaces) > 0 {
		found := false
		for _, ns := range selector.Namespaces {
			if ns == obj.GetNamespace() {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check name selector
	if len(selector.Names) > 0 {
		found := false
		for _, name := range selector.Names {
			if name == obj.GetName() {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check repository name selector
	if selector.RepositoryName != "" {
		switch obj := obj.(type) {
		case *v1alpha1.ResticBackup:
			if obj.Spec.Repository.Name != selector.RepositoryName {
				return false
			}
		case *v1alpha1.ResticRestore:
			if obj.Spec.Repository.Name != selector.RepositoryName {
				return false
			}
		default:
			// This selector is not applicable to other types, so we ignore it.
		}
	}

	return true
}
