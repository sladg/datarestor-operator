package utils

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetResource fetches a Kubernetes resource by its namespace and name,
// returning the populated resource object. It logs an error if an optional logger is provided.
func GetResource[T any](ctx context.Context, c client.Client, namespace, name string, log ...*zap.SugaredLogger) (*T, error) {
	obj := new(T)
	clientObj, ok := any(obj).(client.Object)
	if !ok {
		return nil, fmt.Errorf("%T is not a client.Object", obj)
	}

	err := c.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, clientObj)
	if err != nil {
		if len(log) > 0 && log[0] != nil {
			log[0].Errorw("Failed to get resource", "name", name, "namespace", namespace, "error", err, "type", fmt.Sprintf("%T", obj))
		}
		return nil, err
	}
	return obj, nil
}
