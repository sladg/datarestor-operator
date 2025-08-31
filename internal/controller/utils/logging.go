package utils

import (
	"context"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// LogLevel represents the verbosity level of logging
type LogLevel int

const (
	// LogLevelInfo is for operational status
	LogLevelInfo LogLevel = iota
	// LogLevelDebug is for detailed operational info
	LogLevelDebug
	// LogLevelTrace is for very detailed debug info
	LogLevelTrace
)

// Logger is a wrapper around logr.Logger for structured logging
type Logger struct {
	logger logr.Logger
}

// LoggerFrom creates a structured logger for the given component
func LoggerFrom(ctx context.Context, component string) *Logger {
	return &Logger{
		logger: log.FromContext(ctx).WithName(component),
	}
}

// WithPVC adds PVC information to the logger context
func (l *Logger) WithPVC(pvc corev1.PersistentVolumeClaim) *Logger {
	return &Logger{
		logger: l.logger.WithValues(
			"pvc", pvc.Name,
			"namespace", pvc.Namespace,
		),
	}
}

// WithOperation adds operation information to the logger context
func (l *Logger) WithOperation(op string) *Logger {
	return &Logger{
		logger: l.logger.WithValues("operation", op),
	}
}

// WithBackupID adds backup ID to the logger context
func (l *Logger) WithBackupID(id string) *Logger {
	return &Logger{
		logger: l.logger.WithValues("backupID", id),
	}
}

// WithValues adds arbitrary key-value pairs to the logger context
func (l *Logger) WithValues(keysAndValues ...interface{}) *Logger {
	return &Logger{
		logger: l.logger.WithValues(keysAndValues...),
	}
}

// WithJob adds job information to the logger context
func (l *Logger) WithJob(job interface{}) *Logger {
	switch j := job.(type) {
	case *backupv1alpha1.ResticBackup:
		return &Logger{
			logger: l.logger.WithValues(
				"backup", j.Name,
				"namespace", j.Namespace,
				"phase", j.Status.Phase,
			),
		}
	case *backupv1alpha1.ResticRestore:
		return &Logger{
			logger: l.logger.WithValues(
				"restore", j.Name,
				"namespace", j.Namespace,
				"phase", j.Status.Phase,
			),
		}
	case *backupv1alpha1.BackupConfig:
		return &Logger{
			logger: l.logger.WithValues(
				"config", j.Name,
				"namespace", j.Namespace,
				"schedule", j.Spec.Schedule,
			),
		}
	case *backupv1alpha1.ResticRepository:
		return &Logger{
			logger: l.logger.WithValues(
				"repository", j.Name,
				"namespace", j.Namespace,
				"phase", j.Status.Phase,
			),
		}
	default:
		return l
	}
}

// Info logs informational messages (for operational status)
func (l *Logger) Info(msg string) {
	l.logger.Info(msg)
}

// Debug logs debug messages (for detailed operational info)
func (l *Logger) Debug(msg string) {
	l.logger.V(1).Info(msg)
}

// Trace logs trace messages (for very detailed debug info)
func (l *Logger) Trace(msg string) {
	l.logger.V(2).Info(msg)
}

// Error logs error messages
func (l *Logger) Error(err error, msg string) {
	l.logger.Error(err, msg)
}

// Starting logs the beginning of an operation
func (l *Logger) Starting(what string) {
	l.logger.V(1).Info("starting " + what)
}

// Completed logs the successful completion of an operation
func (l *Logger) Completed(what string) {
	l.logger.Info("completed " + what)
}

// Failed logs the failure of an operation
func (l *Logger) Failed(what string, err error) {
	l.logger.Error(err, "failed "+what)
}

// LogFunc is a function that can be wrapped with logging.
type LogFunc func(ctx context.Context) (ctrl.Result, error)

// WithLogging wraps a function with start, complete, and error logging.
func (l *Logger) WithLogging(ctx context.Context, operationName string, fn LogFunc) (ctrl.Result, error) {
	logger := l.WithValues("operation", operationName)
	logger.Starting(operationName)

	res, err := fn(ctx)

	if err != nil {
		logger.Failed(operationName, err)
	} else {
		logger.Completed(operationName)
	}

	return res, err
}
