package utils

import (
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ShouldPerformBackupFromRepository(schedule string, lastBackupTime metav1.Time, initializedAt metav1.Time) bool {
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

	// If no last backup time, it's time for a backup
	if lastBackupTime.IsZero() {
		return true
	}

	// Get next scheduled time after last backup
	nextBackup := cronSchedule.Next(lastBackupTime.Time)

	// If next scheduled time is in the past, it's time for a backup
	return time.Now().After(nextBackup)
}
