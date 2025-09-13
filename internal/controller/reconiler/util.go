package reconcile_util

import (
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
)

// step centralizes (err, reconcile, period) handling.
// onReconcile is optional; if nil, no update is performed.
func Step(err error, reconcile bool, period time.Duration, onReconcile func() error) (handled bool, res ctrl.Result, retErr error) {
	if err != nil {
		return true, ctrl.Result{}, err
	}
	if reconcile {
		if onReconcile != nil {
			if uerr := onReconcile(); uerr != nil {
				return true, ctrl.Result{}, uerr
			}
		}
		if period == -1 {
			return true, ctrl.Result{}, nil
		}
		return true, ctrl.Result{RequeueAfter: period}, nil
	}
	return false, ctrl.Result{}, nil
}
