package utils

import (
	"context"
	"fmt"
	"reflect"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetResource fetches a Kubernetes resource by its namespace and name,
// returning the populated resource object. It logs an error if an optional logger is provided.
func GetResource[T client.Object](ctx context.Context, c client.Client, namespace, name string) (T, error) {
	var obj T

	// Validate input parameters
	if namespace == "" || name == "" {
		return obj, fmt.Errorf("namespace and name cannot be empty: namespace=%q, name=%q", namespace, name)
	}

	// Additional safety check - ensure the type is not nil
	if reflect.TypeOf(obj) == nil {
		return obj, fmt.Errorf("object type is nil")
	}

	// Create a new instance of the concrete type T points to.
	// For example, if T is *v1.ResticRepository, this creates a new v1.ResticRepository and returns a pointer to it.
	val := reflect.New(reflect.TypeOf(obj).Elem())

	// Additional safety check - ensure the value is not nil
	if val.IsNil() {
		return obj, fmt.Errorf("failed to create new instance of type %T", obj)
	}

	// Assert the new object to the client.Object interface.
	clientObj, ok := val.Interface().(T)
	if !ok {
		// This should not happen if T is a valid client.Object pointer
		return obj, fmt.Errorf("failed to assert type %T to client.Object", val.Interface())
	}

	err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, clientObj)
	if err != nil {
		return obj, err
	}
	return clientObj, nil
}

// ValidateObjectReference validates that an ObjectReference has both Name and Namespace set
func ValidateObjectReference(ref corev1.ObjectReference, refName string) error {
	if ref.Name == "" {
		return fmt.Errorf("%s.Name cannot be empty", refName)
	}
	if ref.Namespace == "" {
		return fmt.Errorf("%s.Namespace cannot be empty", refName)
	}
	return nil
}

// CreateObjectReference creates a validated ObjectReference with proper error handling
func CreateObjectReference(name, namespace string, refName string) (corev1.ObjectReference, error) {
	if name == "" {
		return corev1.ObjectReference{}, fmt.Errorf("%s name cannot be empty", refName)
	}
	if namespace == "" {
		return corev1.ObjectReference{}, fmt.Errorf("%s namespace cannot be empty", refName)
	}

	return corev1.ObjectReference{
		Name:      name,
		Namespace: namespace,
	}, nil
}

// GenerateUniqueName creates a unique name for backup/restore resources
// Format: {type}-{shortUUID} - Keep it simple and clean
// The short UUID ensures uniqueness even in high-frequency scenarios
func GenerateUniqueName(backupConfig, pvc, operationType string) string {
	shortUUID := uuid.New().String()[:6] // Use first 6 characters for cleaner names
	return fmt.Sprintf("%s-%s", operationType, shortUUID)
}
