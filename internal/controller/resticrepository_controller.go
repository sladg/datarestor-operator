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
	if err := utils.AddFinalizer(ctx, r.Deps, repository, constants.ResticRepositoryFinalizer); err != nil {
		log.Errorw("add finalizer failed", "error", err)
		return ctrl.Result{}, err
	}

	// Use a pipeline to handle different phases
	return utils.ProcessSteps(
		// Initial state - check if repository exists and create if needed
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == v1.PhaseUnknown },
			Action: func() (ctrl.Result, error) {
				// Start initialization by transitioning to Pending
				return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhasePending, metav1.Now(), "")
			},
		},
		// Pending state - create init job
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == v1.PhasePending },
			Action: func() (ctrl.Result, error) {
				// Create init job
				job, err := logic.CreateInitJob(ctx, r.Deps, repository)
				if err != nil {
					log.Errorw("Failed to create init job", "error", err)
					return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhaseFailed, metav1.Now(), "")
				}

				return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhaseRunning, metav1.Now(), job.Name)
			},
		},
		// Running state - check job status
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == v1.PhaseRunning },
			Action: func() (ctrl.Result, error) {
				job, err := logic.GetJobBySelector(ctx, r.Deps, repository, "init")
				if err != nil {
					return ctrl.Result{}, err
				}
				if job == nil {
					log.Errorw("No init job found for repository, assuming failure")
					return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhaseFailed, metav1.Now(), "")
				}

				if job.Status.Succeeded > 0 {
					// Verify repository exists
					err := logic.CheckRepositoryExists(ctx, r.Deps, repository)
					if err == nil {
						log.Info("Repository initialized and verified")
						return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhaseCompleted, metav1.Now(), job.Name)
					}
					log.Errorw("Repository check failed after successful init", "error", err)
					return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhaseFailed, metav1.Now(), job.Name)
				}
				if job.Status.Failed > 0 {
					log.Errorw("Init job failed", "reason", job.Status.Conditions[0].Reason, "message", job.Status.Conditions[0].Message)
					return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhaseFailed, metav1.Now(), job.Name)
				}

				log.Debug("Init job still running")
				return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
			},
		},
		// Completed state - run maintenance
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == v1.PhaseCompleted },
			Action: func() (ctrl.Result, error) {
				// Verify repository still exists
				if err := logic.CheckRepositoryExists(ctx, r.Deps, repository); err != nil {
					log.Errorw("Repository check failed during maintenance", "error", err)
					return logic.UpdateRepoStatus(ctx, r.Deps, repository, v1.PhaseFailed, metav1.Now(), "")
				}
				return logic.HandleRepoMaintenance(ctx, r.Deps, repository)
			},
		},
		// Failed state - wait for manual intervention
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == v1.PhaseFailed },
			Action: func() (ctrl.Result, error) {
				return logic.HandleRepoFailed(ctx, r.Deps, repository)
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
