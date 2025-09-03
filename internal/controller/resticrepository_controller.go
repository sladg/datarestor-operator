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

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	constants "github.com/sladg/datarestor-operator/internal/constants"
	utils "github.com/sladg/datarestor-operator/internal/controller/utils"
	logic "github.com/sladg/datarestor-operator/internal/logic/resticrepository"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
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

// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrepositories,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrepositories/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticrepositories/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=backup.datarestor-operator.com,resources=resticbackups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile handles ResticRepository initialization and maintenance
func (r *ResticRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Deps.Logger.With("name", req.Name, "namespace", req.Namespace)

	// Fetch the ResticRepository instance
	repository, err := utils.GetResource[v1.ResticRepository](ctx, r.Deps.Client, req.Namespace, req.Name, log)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			log.Info("ResticRepository resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	// Handle deletion
	if repository.DeletionTimestamp != nil {
		if repository.Status.Phase != v1.PhaseDeletion {
			return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhaseDeletion, metav1.Now(), "")
		}
	} else {
		// Add finalizer if not present
		if err := utils.AddFinalizer(ctx, r.Deps, repository, constants.ResticRepositoryFinalizer); err != nil {
			log.Errorw("add finalizer failed", "error", err)
			return ctrl.Result{}, err
		}
	}

	// Main reconciliation logic based on phase
	switch repository.Status.Phase {
	case v1.PhaseUnknown:
		return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhasePending, metav1.Now(), "")
	case v1.PhasePending:
		return logic.HandleRepoPending(ctx, r.Deps, repository)
	case v1.PhaseRunning:
		return logic.HandleRepoRunning(ctx, r.Deps, repository)
	case v1.PhaseCompleted:
		return logic.HandleRepoMaintenance(ctx, r.Deps, repository)
	case v1.PhaseFailed:
		return logic.HandleRepoFailed(ctx, r.Deps, repository)
	case v1.PhaseDeletion:
		return logic.HandleRepositoryDeletion(ctx, r.Deps, repository)
	default:
		log.Errorw("Unknown phase", "phase", repository.Status.Phase)
		return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
	}
}

// SetupWithManager sets up the controller with the Manager
func (r *ResticRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ResticRepository{}).
		Complete(r)
}
