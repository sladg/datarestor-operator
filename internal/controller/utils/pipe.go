package utils

import (
	ctrl "sigs.k8s.io/controller-runtime"
)

// ConditionFunc is a function that returns true if a condition is met.
type ConditionFunc func() bool

// ActionFunc is a function that performs an action and returns a result and an error.
type ActionFunc func() (ctrl.Result, error)

// Step represents a single step in a processing pipeline.
// If the Condition is met, the Action is executed.
type Step struct {
	Condition ConditionFunc
	Action    ActionFunc
}

// ProcessSteps executes a series of steps. It stops and returns the result
// of the first step whose condition is met.
func ProcessSteps(steps ...Step) (ctrl.Result, error) {
	for _, step := range steps {
		if step.Condition() {
			return step.Action()
		}
	}
	// If no condition was met, return a default result.
	return ctrl.Result{}, nil
}
