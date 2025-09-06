package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type OperationType string

const (
	FinalizerOperation  OperationType = "finalizer"
	AnnotationOperation OperationType = "annotation"
)

type BulkOperationParams struct {
	Objects   []corev1.ObjectReference
	Map       map[string]*string // Map of annotation/finalizer key to value (nil means remove)
	Operation OperationType
}

func setFinalizers(ctx context.Context, deps *Dependencies, obj client.Object, mapFinalizers map[string]*string) error {
	finalizers := obj.GetFinalizers()

	if finalizers == nil {
		finalizers = make([]string, 0)
	}

	for fianlizer, value := range mapFinalizers {
		if value == nil {
			for i, f := range finalizers {
				if f == fianlizer {
					finalizers = append(finalizers[:i], finalizers[i+1:]...)
					break
				}
			}
		} else {
			finalizers = append(finalizers, fianlizer)
		}
	}

	obj.SetFinalizers(finalizers)

	return deps.Update(ctx, obj)
}

func setAnnotations(ctx context.Context, deps *Dependencies, obj client.Object, mapAnnotations map[string]*string) error {
	annotations := obj.GetAnnotations()

	if annotations == nil {
		annotations = make(map[string]string)
	}

	// Set the annotation
	for annotation, value := range mapAnnotations {
		if value == nil {
			delete(annotations, annotation)
		} else {
			annotations[annotation] = *value
		}
	}
	obj.SetAnnotations(annotations)

	return deps.Update(ctx, obj)
}

func ApplyBulkFinalizer(ctx context.Context, deps *Dependencies, params BulkOperationParams) error {
	for _, obj := range params.Objects {
		var resource client.Object
		if err := deps.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}, resource); err != nil {
			return fmt.Errorf("failed to get %s: %w", obj.Name, err)
		}

		if err := setFinalizers(ctx, deps, resource, params.Map); err != nil {
			return fmt.Errorf("failed to set finalizers for %s: %w", obj.Name, err)
		}
	}

	return nil
}

func ApplyBulkAnnotation(ctx context.Context, deps *Dependencies, params BulkOperationParams) error {
	for _, obj := range params.Objects {
		var resource client.Object
		if err := deps.Get(ctx, client.ObjectKey{Namespace: obj.Namespace, Name: obj.Name}, resource); err != nil {
			return fmt.Errorf("failed to get %s: %w", obj.Name, err)
		}

		if err := setAnnotations(ctx, deps, resource, params.Map); err != nil {
			return fmt.Errorf("failed to set annotations for %s: %w", obj.Name, err)
		}
	}

	return nil
}

func CheckFinalizerOnPVC(ctx context.Context, deps *Dependencies, pvc corev1.ObjectReference, finalizer string) bool {
	var resource client.Object
	if err := deps.Get(ctx, client.ObjectKey{Namespace: pvc.Namespace, Name: pvc.Name}, resource); err != nil {
		return true
	}

	return controllerutil.ContainsFinalizer(resource, finalizer)
}

func SetOwnFinalizer(ctx context.Context, deps *Dependencies, obj client.Object, finalizer string) error {
	controllerutil.AddFinalizer(obj, finalizer)

	if err := deps.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to set finalizer on %s: %w", obj.GetName(), err)
	}

	return nil
}

func RemoveOwnFinalizer(ctx context.Context, deps *Dependencies, obj client.Object, finalizer string) error {
	controllerutil.RemoveFinalizer(obj, finalizer)

	if err := deps.Update(ctx, obj); err != nil {
		return fmt.Errorf("failed to remove finalizer from %s: %w", obj.GetName(), err)
	}

	return nil
}

func RemoveOwnAnnotation(ctx context.Context, deps *Dependencies, obj client.Object, annotation string) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	delete(annotations, annotation)
	return deps.Update(ctx, obj)
}
