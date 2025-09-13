package utils

import (
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ShouldPerformScheduledOperation(schedule string, lastRunTime metav1.Time, initializedAt metav1.Time) bool {
	if initializedAt.IsZero() {
		return false
	}

	if schedule == "" {
		return false
	}

	cronSchedule, err := cron.ParseStandard(schedule)
	if err != nil {
		return false
	}

	// If no last run time, it's time for the operation
	if lastRunTime.IsZero() {
		return true
	}

	// Get next scheduled time after last run
	nextRun := cronSchedule.Next(lastRunTime.Time)

	// If next scheduled time is in the past, it's time for the operation
	return time.Now().After(nextRun)
}
