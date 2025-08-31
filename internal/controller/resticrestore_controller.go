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
)

// ResticRestoreReconciler reconciles a ResticRestore object
type ResticRestoreReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Config *rest.Config
}

//+kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrestores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrestores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrestores/finalizers,verbs=update
//+kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticbackups,verbs=get;list;watch
//+kubebuilder:rbac:groups=backup.autorestore-backup-operator.com,resources=resticrepositories,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps,resources=deployments;statefulsets;daemonsets,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResticRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := utils.LoggerFrom(ctx, "restore").
		WithValues("resticrestore", req.NamespacedName)

	// Fetch the ResticRestore instance
	resticRestore := &backupv1alpha1.ResticRestore{}
	if err := r.Get(ctx, req.NamespacedName, resticRestore); err != nil {
		if errors.IsNotFound(err) {
			logger.Debug("ResticRestore resource not found, assuming it was deleted")
			return ctrl.Result{}, nil
		}
		logger.Failed("fetch ResticRestore", err)
		return ctrl.Result{}, err
	}

	logger = logger.WithValues("restore", resticRestore.Name, "phase", resticRestore.Status.Phase)

	// Handle deletion
	if resticRestore.DeletionTimestamp != nil {
		logger.Starting("finalizer processing")
		return stubs.HandleResticRestoreDeletion(ctx, r.Client, r.Scheme, resticRestore)
	}

	// Add finalizer if missing
	if !controllerutil.ContainsFinalizer(resticRestore, constants.ResticRestoreFinalizer) {
		logger.Debug("Adding finalizer")
		controllerutil.AddFinalizer(resticRestore, constants.ResticRestoreFinalizer)
		return ctrl.Result{}, r.Update(ctx, resticRestore)
	}

	// Handle different phases
	return utils.ProcessSteps(
		utils.Step{
			Condition: func() bool { return resticRestore.Status.Phase == "" || resticRestore.Status.Phase == "Pending" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleRestorePending(ctx, r.Client, r.Scheme, resticRestore)
			},
		},
		utils.Step{
			Condition: func() bool { return resticRestore.Status.Phase == "Running" },
			Action: func() (ctrl.Result, error) {
				return stubs.HandleRestoreRunning(ctx, r.Client, r.Scheme, resticRestore)
			},
		},
		utils.Step{
			Condition: func() bool {
				return resticRestore.Status.Phase == "Completed" || resticRestore.Status.Phase == "Failed"
			},
			Action: func() (ctrl.Result, error) {
				return ctrl.Result{}, nil
			},
		},
		utils.Step{
			Condition: func() bool { return true }, // Default case
			Action: func() (ctrl.Result, error) {
				logger.WithValues("unknownPhase", resticRestore.Status.Phase).Info("Unknown phase, treating as pending")
				return stubs.HandleRestorePending(ctx, r.Client, r.Scheme, resticRestore)
			},
		},
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResticRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&backupv1alpha1.ResticRestore{}).
		Complete(r)
}

// NewResticRestoreReconciler creates a new ResticRestoreReconciler
func NewResticRestoreReconciler(client client.Client, scheme *runtime.Scheme, config *rest.Config) *ResticRestoreReconciler {
	return &ResticRestoreReconciler{
		Client: client,
		Scheme: scheme,
		Config: config,
	}
}
