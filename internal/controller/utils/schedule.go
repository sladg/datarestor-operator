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

// GetNextBackupTime returns the next scheduled backup time for debugging purposes
func GetNextBackupTime(schedule string, lastBackupTime *time.Time) *time.Time {
	if schedule == "" {
		return nil
	}

	cronSchedule, err := cron.ParseStandard(schedule)
	if err != nil {
		return nil
	}

	var nextTime time.Time
	if lastBackupTime != nil {
		nextTime = cronSchedule.Next(*lastBackupTime)
	} else {
		nextTime = cronSchedule.Next(time.Now())
	}

	return &nextTime
}
