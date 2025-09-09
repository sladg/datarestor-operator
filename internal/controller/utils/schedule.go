package utils

import (
	"time"

	"github.com/robfig/cron/v3"
	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
)

func ShouldPerformBackupFromRepository(repoSpec v1.RepositorySpec) bool {
	schedule := repoSpec.BackupSchedule
	lastBackupTime := repoSpec.Status.LastScheduledBackupRun

	if repoSpec.Status.InitializedAt == nil {
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
	if lastBackupTime == nil {
		return true
	}

	// Get next scheduled time after last backup
	nextBackup := cronSchedule.Next(lastBackupTime.Time)

	// If next scheduled time is in the past, it's time for a backup
	return time.Now().After(nextBackup)
}
