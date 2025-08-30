package controller

import (
	"context"
	"fmt"
	"time"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
)

// calculateNextBackupTime calculates the next backup time based on cron schedule
func (r *BackupConfigReconciler) calculateNextBackupTime(cronExpression string) (time.Time, error) {
	if cronExpression == "" {
		return time.Time{}, fmt.Errorf("cron expression is empty")
	}

	schedule, err := r.cronParser.Parse(cronExpression)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse cron expression: %w", err)
	}

	// Calculate next backup time from now (this is used for status display)
	return schedule.Next(time.Now()), nil
}

// calculateNextReconcile calculates when the next reconciliation should occur
func (r *BackupConfigReconciler) calculateNextReconcile(pvcBackup *backupv1alpha1.BackupConfig) (time.Duration, error) {
	logger := LoggerFrom(context.Background(), "schedule").
		WithValues("name", pvcBackup.Name)

	logger.Starting("calculate next reconcile")

	// If no cron schedule is configured, use a reasonable default
	if pvcBackup.Spec.Schedule.Cron == "" {
		logger.Debug("No cron schedule, using default interval")
		return 1 * time.Hour, nil
	}

	// Parse the cron schedule
	schedule, err := r.cronParser.Parse(pvcBackup.Spec.Schedule.Cron)
	if err != nil {
		logger.Failed("parse cron expression", err)
		// If cron parsing fails, use default interval
		return 1 * time.Hour, nil
	}

	now := time.Now()

	// If no last backup, schedule for the next backup time
	if pvcBackup.Status.LastBackup == nil {
		nextBackup := schedule.Next(now)
		interval := nextBackup.Sub(now)
		logger.WithValues("interval", interval).Debug("First backup, scheduling next")
		return interval, nil
	}

	// Calculate next backup time from the last backup
	lastBackup := pvcBackup.Status.LastBackup.Time
	nextBackup := schedule.Next(lastBackup)

	if nextBackup.After(now) {
		// Schedule reconciliation for the next backup time
		interval := nextBackup.Sub(now)
		logger.WithValues(
			"lastBackup", lastBackup,
			"nextBackup", nextBackup,
			"interval", interval,
		).Debug("Scheduling next backup")
		return interval, nil
	}

	// If next backup time has passed, schedule for a short interval
	logger.Debug("Next backup time has passed, using short interval")
	return 5 * time.Minute, nil
}
