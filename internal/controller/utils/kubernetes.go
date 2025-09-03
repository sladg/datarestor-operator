package utils

import (
	"context"
	"fmt"
	"reflect"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetResource fetches a Kubernetes resource by its namespace and name,
// returning the populated resource object. It logs an error if an optional logger is provided.
func GetResource[T client.Object](ctx context.Context, c client.Client, namespace, name string) (T, error) {
	var obj T
	// Create a new instance of the concrete type T points to.
	// For example, if T is *v1.ResticRepository, this creates a new v1.ResticRepository and returns a pointer to it.
	val := reflect.New(reflect.TypeOf(obj).Elem())

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
