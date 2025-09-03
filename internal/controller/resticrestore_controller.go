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
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/logic/resticrestore"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewResticRestoreReconciler(deps *utils.Dependencies) *ResticRestoreReconciler {
	return &ResticRestoreReconciler{
		Deps: deps,
	}
}

type ResticRestoreReconciler struct {
	Deps *utils.Dependencies
}

// +kubebuilder:rbac:groups=backup.autorestore.com,resources=resticrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.autorestore.com,resources=resticrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.autorestore.com,resources=resticrestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=backup.autorestore.com,resources=resticbackups,verbs=get;list;watch
// +kubebuilder:rbac:groups=backup.autorestore.com,resources=resticrepositories,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;patch;update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;patch;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ResticRestoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Deps.Logger.With("name", req.Name, "namespace", req.Namespace)

	// Fetch the ResticRestore instance
	resticRestore := &v1.ResticRestore{}
	if err := r.Deps.Get(ctx, req.NamespacedName, resticRestore); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ResticRestore resource not found. Ignoring...")
			return ctrl.Result{}, nil
		}
		log.Errorw("Failed to get ResticRestore", "error", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if resticRestore.DeletionTimestamp != nil {
		resticRestore.Status.Phase = v1.PhaseDeletion
		if err := r.Deps.Status().Update(ctx, resticRestore); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
	}

	p := resticRestore.Status.Phase

	return utils.ProcessSteps(
		utils.Step{
			Condition: func() bool { return p == v1.PhaseUnknown || p == v1.PhasePending },
			Action:    func() (ctrl.Result, error) { return resticrestore.HandleRestorePending(ctx, r.Deps, resticRestore) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseRunning },
			Action:    func() (ctrl.Result, error) { return resticrestore.HandleRestoreRunning(ctx, r.Deps, resticRestore) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseFailed },
			Action:    func() (ctrl.Result, error) { return resticrestore.HandleRestoreFailed(ctx, r.Deps, resticRestore) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseCompleted },
			Action:    func() (ctrl.Result, error) { return resticrestore.HandleRestoreCompleted(ctx, r.Deps, resticRestore) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseDeletion },
			Action:    func() (ctrl.Result, error) { return resticrestore.HandleRestoreDeletion(ctx, r.Deps, resticRestore) },
		},
		utils.Step{
			Condition: func() bool { return true }, // Default case
			Action: func() (ctrl.Result, error) {
				log.Infow("Unknown phase, treating as pending", "unknownPhase", p)
				return resticrestore.HandleRestorePending(ctx, r.Deps, resticRestore)
			},
		},
	)
}

// SetupWithManager sets up the controller with the Manager.
func (r *ResticRestoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ResticRestore{}).
		Complete(r)
}
