package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/cenkalti/backoff/v4"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// retryWithBackoff executes a function with exponential backoff retry logic.
// It retries on conflict errors (optimistic concurrency control failures).
func retryWithBackoff(ctx context.Context, operation func() error) error {
	backoffConfig := backoff.NewExponentialBackOff()
	backoffConfig.InitialInterval = 250 * time.Millisecond
	backoffConfig.MaxInterval = 1 * time.Second
	backoffConfig.MaxElapsedTime = 10 * time.Second
	backoffConfig.Multiplier = 2.0

	return backoff.Retry(func() error {
		err := operation()
		if err != nil && errors.IsConflict(err) {
			// Retry on conflict errors (optimistic concurrency control)
			return err
		}
		// Don't retry on other errors
		return backoff.Permanent(err)
	}, backoff.WithContext(backoffConfig, ctx))
}

// UpdateWithRetry updates a Kubernetes object with retry logic for conflict errors.
// It refreshes the object from the API server before updating to handle race conditions.
// The updateFunc parameter specifies whether to update the object or just its status.
func UpdateWithRetry(ctx context.Context, deps *Dependencies, obj client.Object, updateFunc func(ctx context.Context, obj client.Object) error) error {
	// Get the object key for refreshing
	objKey := client.ObjectKeyFromObject(obj)

	return retryWithBackoff(ctx, func() error {
		// Refresh the object from the API server to get the latest version
		if err := deps.Get(ctx, objKey, obj); err != nil {
			return fmt.Errorf("failed to refresh object %s %s: %w", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
		}

		// Perform the update operation
		if err := updateFunc(ctx, obj); err != nil {
			return fmt.Errorf("failed to update %s %s: %w", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
		}

		return nil
	})
}

// UpdateStatusWithRetry updates the status of a Kubernetes object with retry logic for conflict errors.
// It refreshes the object from the API server before updating to handle race conditions.
func UpdateStatusWithRetry(ctx context.Context, deps *Dependencies, obj client.Object) error {
	return UpdateWithRetry(ctx, deps, obj, func(ctx context.Context, obj client.Object) error {
		return deps.Status().Update(ctx, obj)
	})
}

// UpdateObjectWithRetry updates a Kubernetes object with retry logic for conflict errors.
// It refreshes the object from the API server before updating to handle race conditions.
func UpdateObjectWithRetry(ctx context.Context, deps *Dependencies, obj client.Object) error {
	return UpdateWithRetry(ctx, deps, obj, func(ctx context.Context, obj client.Object) error {
		return deps.Update(ctx, obj)
	})
}
