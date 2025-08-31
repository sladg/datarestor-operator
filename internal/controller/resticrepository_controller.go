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

	"github.com/robfig/cron/v3"
	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	"github.com/sladg/autorestore-backup-operator/internal/constants"
	"github.com/sladg/autorestore-backup-operator/internal/controller/utils"
	"github.com/sladg/autorestore-backup-operator/internal/stubs"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ResticRepositoryReconciler reconciles a ResticRepository object
type ResticRepositoryReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Config     *rest.Config
	cronParser cron.Parser
}

// NewResticRepositoryReconciler creates a new ResticRepositoryReconciler
func NewResticRepositoryReconciler(client client.Client, scheme *runtime.Scheme, config *rest.Config) *ResticRepositoryReconciler {
	return &ResticRepositoryReconciler{
		Client:     client,
		Scheme:     scheme,
		Config:     config,
		cronParser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}
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
	logger := log.FromContext(ctx).WithValues("repository", req.Name)
	logger.Info("reconcile")

	// Fetch the ResticRepository instance
	repository := &backupv1alpha1.ResticRepository{}
	if err := r.Get(ctx, req.NamespacedName, repository); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		logger.Error(err, "get repository")
		return ctrl.Result{}, err
	}

	// Handle deletion
	if repository.DeletionTimestamp != nil {
		return stubs.HandleRepoDeletion(ctx, r.Client, repository)
	}

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(repository, constants.ResticRepositoryFinalizer) {
		controllerutil.AddFinalizer(repository, constants.ResticRepositoryFinalizer)
		if err := r.Update(ctx, repository); err != nil {
			logger.Error(err, "add finalizer")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Handle repository lifecycle based on current phase
	return utils.ProcessSteps(
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == "" || repository.Status.Phase == "Unknown" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleRepoInitialization(ctx, r.Client, r.Scheme, repository)
			},
		},
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == "Initializing" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleRepoInitializationStatus(ctx, r.Client, r.Scheme, repository)
			},
		},
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == "Ready" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleRepoMaintenance(ctx, r.Client, r.Scheme, repository, r.cronParser)
			},
		},
		utils.Step{
			Condition: func() bool { return repository.Status.Phase == "Failed" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleRepoFailed(ctx, r.Client, r.Scheme, repository)
			},
		},
		utils.Step{
			Condition: func() bool { return true }, // Default case
			Action: func() (ctrl.Result, error) {
				utils.LoggerFrom(ctx, "restic-repository").
					WithValues("name", req.Name, "namespace", req.Namespace).
					WithValues("phase", repository.Status.Phase).Debug("Unknown phase")
				return ctrl.Result{RequeueAfter: constants.DefaultRequeueInterval}, nil
			},
		},
	)
}

// SetupWithManager sets up the controller with the Manager
func (r *ResticRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.ResticRepository{}).
		Complete(r)
}
