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
	constants "github.com/sladg/datarestor-operator/internal/constants"
	utils "github.com/sladg/datarestor-operator/internal/controller/utils"
	"github.com/sladg/datarestor-operator/internal/logic/resticrepository"
	"k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
)

func NewResticRepositoryReconciler(deps *utils.Dependencies) (*ResticRepositoryReconciler, error) {
	return &ResticRepositoryReconciler{
		Deps: deps,
	}, nil
}

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

func (r *ResticRepositoryReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Deps.Logger.With("name", req.Name, "namespace", req.Namespace)

	// Fetch the ResticRepository instance
	resticRepo := &v1.ResticRepository{}
	if err := r.Deps.Get(ctx, req.NamespacedName, resticRepo); err != nil {
		if errors.IsNotFound(err) {
			log.Info("ResticRepository resource not found. Ignoring...")
			return ctrl.Result{}, nil
		}
		log.Errorw("Failed to get ResticRepository", "error", err)
		return ctrl.Result{}, err
	}

	// Handle deletion
	if resticRepo.DeletionTimestamp != nil {
		resticRepo.Status.Phase = v1.PhaseDeletion
		if err := r.Deps.Status().Update(ctx, resticRepo); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, nil
	}

	p := resticRepo.Status.Phase

	return utils.ProcessSteps(
		utils.Step{
			Condition: func() bool {
				return p == v1.PhaseUnknown || p == v1.PhasePending
			},
			Action: func() (ctrl.Result, error) { return resticrepository.HandleRepoPending(ctx, r.Deps, resticRepo) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseRunning },
			Action:    func() (ctrl.Result, error) { return resticrepository.HandleRepoRunning(ctx, r.Deps, resticRepo) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseFailed },
			Action:    func() (ctrl.Result, error) { return resticrepository.HandleRepoFailed(ctx, r.Deps, resticRepo) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseCompleted },
			Action:    func() (ctrl.Result, error) { return resticrepository.HandleRepoCompleted(ctx, r.Deps, resticRepo) },
		},
		utils.Step{
			Condition: func() bool { return p == v1.PhaseDeletion },
			Action:    func() (ctrl.Result, error) { return resticrepository.HandleRepoDeletion(ctx, r.Deps, resticRepo) },
		},
		utils.Step{
			Condition: func() bool { return true }, // Default case
			Action: func() (ctrl.Result, error) {
				r.Deps.Logger.Infow("Unknown or unhandled phase", "phase", p)
				return ctrl.Result{}, fmt.Errorf("unknown or unhandled phase: %s", p)
			},
		},
	)
}

func (r *ResticRepositoryReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ResticRepository{}).
		Complete(r)
}
