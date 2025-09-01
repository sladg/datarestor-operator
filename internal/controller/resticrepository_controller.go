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

	v1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	constants "github.com/sladg/autorestore-backup-operator/internal/constants"
	utils "github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	logic "github.com/sladg/autorestore-backup-operator/internal/logic/resticrepository"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// NewResticRepositoryReconciler creates a new ResticRepositoryReconciler
func NewResticRepositoryReconciler(deps *utils.Dependencies) (*ResticRepositoryReconciler, error) {
	return &ResticRepositoryReconciler{
		Deps: deps,
	}, nil
}

// ResticRepositoryReconciler reconciles a ResticRepository object
type ResticRepositoryReconciler struct {
	Deps *utils.Dependencies
}

// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrepositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrepositories/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile handles ResticRepository initialization and maintenance
func (r *ResticRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Deps.Logger.With("name", req.Name, "namespace", req.Namespace)

	// Fetch the ResticRepository instance
	repository := &v1.ResticRepository{}
	if err := r.Deps.Client.Get(ctx, req.NamespacedName, repository); err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("ResticRepository resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		log.Errorw("Failed to get ResticRepository", "error", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if repository.DeletionTimestamp != nil {
		return logic.HandleRepoDeletion(ctx, r.Deps, repository)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(repository, constants.ResticRepositoryFinalizer) {
		controllerutil.AddFinalizer(repository, constants.ResticRepositoryFinalizer)
		if err := r.Deps.Client.Update(ctx, repository); err != nil {
			log.Errorw("add finalizer failed", "error", err)
			return ctrl.Result{}, err
		}
	}

	// Use a pipeline to handle different phases
	return utils.ProcessSteps(
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == "" },
			Action: func() (ctrl.Result, error) {
				return logic.HandleRepoInitialization(ctx, r.Deps, repository)
			},
		},
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == v1.PhasePending },
			Action: func() (ctrl.Result, error) {
				return logic.HandleRepoInitializationStatus(ctx, r.Deps, repository)
			},
		},
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == v1.PhaseCompleted },
			Action: func() (ctrl.Result, error) {
				return logic.HandleRepoMaintenance(ctx, r.Deps, repository)
			},
		},
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == v1.PhaseFailed },
			Action: func() (ctrl.Result, error) {
				return logic.HandleRepoFailed(ctx, r.Deps, repository)
			},
		},
		utils.Step{
			Condition: func() bool { return true }, // Default case
			Action: func() (ctrl.Result, error) {
				log.Info("unknown phase; initialize", "phase", repository.Status.Phase)
				return logic.HandleRepoInitialization(ctx, r.Deps, repository)
			},
		},
	)
}

// SetupWithManager sets up the controller with the Manager
func (r *ResticRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ResticRepository{}).
		Complete(r)
}
