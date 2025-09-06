/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/logic/resticrestore"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

func NewResticRestoreReconciler(deps *utils.Dependencies) *ResticRestoreReconciler {
	return &ResticRestoreReconciler{
		Deps: deps,
	}
}

type ResticRestoreReconciler struct {
	Deps *utils.Dependencies
}

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups,verbs=get;list;watch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrepositories,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=apps,resources=replicasets,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;patch;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResticRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Deps.Logger.Named("[ResticRestore]").With("name", req.Name, "namespace", req.Namespace)

	// Create dependencies with the named logger for inheritance
	deps := *r.Deps
	deps.Logger = logger

	// Fetch the ResticRestore instance
	resticRestore := &v1.ResticRestore{}
	if err := r.Deps.Get(ctx, req.NamespacedName, resticRestore); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("ResticRestore resource not found. Ignoring...")
			return ctrl.Result{}, nil
		}
		logger.Errorw("Failed to get ResticRestore", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if resticRestore.DeletionTimestamp != nil {
		return resticrestore.HandleRestoreDeletion(ctx, &deps, resticRestore)
	}

	// Handle different phases with dynamic phase checking
	switch resticRestore.Status.Phase {
	case v1.PhaseUnknown:
		return resticrestore.HandleRestoreUnknown(ctx, &deps, resticRestore)
	case v1.PhasePending:
		return resticrestore.HandleRestorePending(ctx, &deps, resticRestore)
	case v1.PhaseRunning:
		return resticrestore.HandleRestoreRunning(ctx, &deps, resticRestore)
	case v1.PhaseFailed:
		return resticrestore.HandleRestoreFailed(ctx, &deps, resticRestore)
	case v1.PhaseCompleted:
		return resticrestore.HandleRestoreCompleted(ctx, &deps, resticRestore)
	case v1.PhaseDeletion:
		return resticrestore.HandleRestoreDeletion(ctx, &deps, resticRestore)
	default:
		logger.Errorw("Unknown phase", "phase", resticRestore.Status.Phase)
		return ctrl.Result{}, fmt.Errorf("unknown phase: %s", resticRestore.Status.Phase)
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResticRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ResticRestore{}).
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		Complete(r)
}
