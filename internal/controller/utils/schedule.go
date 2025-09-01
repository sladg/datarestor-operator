package utils

import (
	"time"

	"github.com/robfig/cron/v3"
)

// ShouldPerformBackup checks if a backup should be performed based on the cron schedule
func ShouldPerformBackup(schedule string, lastBackupTime *time.Time) bool {
	if schedule == "" {
		return false
	}

	cronSchedule, err := cron.ParseStandard(schedule)
	if err != nil {
		return false
	}

	// If no last backup time, it's time for a backup
	if lastBackupTime == nil {
		return true
	}

	// Get next scheduled time after last backup
	nextBackup := cronSchedule.Next(*lastBackupTime)

	// If next scheduled time is in the past, it's time for a backup
	return time.Now().After(nextBackup)
}
