package controller

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
)

// LogLevel represents the verbosity level of logging
type LogLevel int

const (
	// LogLevelInfo is for operational status
	LogLevelInfo LogLevel = iota
	// LogLevelDebug is for detailed operational info
	LogLevelDebug
	// LogLevelTrace is for implementation details
	LogLevelTrace
)

// LoggerFrom creates a logger with common fields from context
func LoggerFrom(ctx context.Context, component string) *Logger {
	return &Logger{
		logger: log.FromContext(ctx).WithName(component),
	}
}

// Logger wraps logr.Logger to provide consistent logging patterns
type Logger struct {
	logger logr.Logger
	fields map[string]interface{}
}

// WithPVC adds PVC information to the logger
func (l *Logger) WithPVC(pvc corev1.PersistentVolumeClaim) *Logger {
	l.logger = l.logger.WithValues(
		"pvc", pvc.Name,
		"namespace", pvc.Namespace,
	)
	return l
}

// WithOperation adds operation type to the logger
func (l *Logger) WithOperation(op string) *Logger {
	l.logger = l.logger.WithValues("operation", op)
	return l
}

// WithBackupID adds backup ID to the logger
func (l *Logger) WithBackupID(id string) *Logger {
	l.logger = l.logger.WithValues("backup_id", id)
	return l
}

// WithValues adds arbitrary key-value pairs to the logger
func (l *Logger) WithValues(keysAndValues ...interface{}) *Logger {
	l.logger = l.logger.WithValues(keysAndValues...)
	return l
}

// WithJob adds backup job information to the logger
func (l *Logger) WithJob(job interface{}) *Logger {
	switch v := job.(type) {
	case *storagev1alpha1.PVCBackupJob:
		values := []interface{}{
			"job_name", v.Name,
			"job_namespace", v.Namespace,
			"pvc", v.Spec.PVCRef.Name,
			"target", v.Spec.BackupTarget.Name,
			"job_phase", v.Status.Phase,
		}

		if v.Status.ResticID != "" {
			values = append(values, "restic_id", v.Status.ResticID)
		}

		if v.Status.Size > 0 {
			values = append(values, "size", v.Status.Size)
		}

		if v.Status.CompletionTime != nil {
			values = append(values, "completion_time", v.Status.CompletionTime.Time)
		}

		return l.WithValues(values...)

	case *storagev1alpha1.PVCBackupRestoreJob:
		values := []interface{}{
			"job_name", v.Name,
			"job_namespace", v.Namespace,
			"pvc", v.Spec.PVCRef.Name,
			"target", v.Spec.BackupTarget.Name,
			"job_phase", v.Status.Phase,
			"restore_type", v.Spec.RestoreType,
			"restore_status", v.Status.RestoreStatus,
		}

		if v.Status.RestoredBackupID != "" {
			values = append(values, "backup_id", v.Status.RestoredBackupID)
		}

		if v.Status.DataRestored > 0 {
			values = append(values, "data_restored", v.Status.DataRestored)
		}

		if v.Status.CompletionTime != nil {
			values = append(values, "completion_time", v.Status.CompletionTime.Time)
		}

		return l.WithValues(values...)

	default:
		// Fallback for unknown types
		return l.WithValues("job", job)
	}
}

// Info logs at info level
func (l *Logger) Info(msg string) {
	l.logger.Info(msg)
}

// Debug logs at debug level (V=1)
func (l *Logger) Debug(msg string) {
	l.logger.V(1).Info(msg)
}

// Trace logs at trace level (V=2)
func (l *Logger) Trace(msg string) {
	l.logger.V(2).Info(msg)
}

// Error logs an error with message
func (l *Logger) Error(err error, msg string) {
	l.logger.Error(err, msg)
}

// Starting logs the start of an operation
func (l *Logger) Starting(what string) {
	l.Info("Starting " + what)
}

// Completed logs the completion of an operation
func (l *Logger) Completed(what string) {
	l.Info("Completed " + what)
}

// Failed logs the failure of an operation
func (l *Logger) Failed(what string, err error) {
	l.Error(err, "Failed "+what)
}
