package reconcile_util

import (
	"time"

	"github.com/sladg/datarestor-operator/internal/constants"
	ctrl "sigs.k8s.io/controller-runtime"
)

// step centralizes (err, reconcile, period) handling.
// onReconcile is optional; if nil, no update is performed.
func Step(err error, reconcile bool, period time.Duration, onReconcile func() error) (handled bool, res ctrl.Result, retErr error) {
	if err != nil {
		return true, ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, err
	}
	if reconcile {
		if onReconcile != nil {
			if uerr := onReconcile(); uerr != nil {
				return true, ctrl.Result{RequeueAfter: constants.ImmediateRequeueInterval}, uerr
			}
		}
		return true, ctrl.Result{RequeueAfter: period}, nil
	}
	return false, ctrl.Result{}, nil
}
