package constants

import "time"

// Domain for the operator
const OperatorDomain = "backup.autorestore-backup-operator.com"

// Labels used for resource identification
const (
	// Resource identifiers
	LabelPVCName = OperatorDomain + "/pvc-name" // Name of related PVC
	LabelJobType = OperatorDomain + "/job-type" // Type of job (backup, restore, repository)

	// Job types
	JobTypeBackup     = "backup"     // Backup job
	JobTypeRestore    = "restore"    // Restore job
	JobTypeRepository = "repository" // Repository maintenance job
)

// Job name patterns
const (
	BackupJobPattern     = "%s-backup-%s"     // {backupName}-backup-{timestamp}
	RestoreJobPattern    = "%s-restore-%s"    // {restoreName}-restore-{timestamp}
	RepositoryJobPattern = "%s-repository-%s" // {repoName}-repository-{timestamp}
)

// Finalizers ensure proper cleanup of resources
const (
	BackupConfigFinalizer     = OperatorDomain + "/backupconfig-finalizer"     // BackupConfig finalizer
	ResticRepositoryFinalizer = OperatorDomain + "/resticrepository-finalizer" // ResticRepository finalizer
	ResticBackupFinalizer     = OperatorDomain + "/resticbackup-finalizer"     // ResticBackup finalizer
	ResticRestoreFinalizer    = OperatorDomain + "/resticrestore-finalizer"    // ResticRestore finalizer
	PVCRestoreFinalizer       = OperatorDomain + "/pvc-restore-in-progress"    // PVC finalizer during restore
	WorkloadFinalizer         = OperatorDomain + "/workload-finalizer"         // Workload finalizer
)

// Annotations for resource state and operations
const (
	// Operation triggers
	AnnotationManualBackup  = OperatorDomain + "/manual-backup"  // Trigger manual backup
	AnnotationManualRestore = OperatorDomain + "/manual-restore" // Trigger manual restore
	AnnotationRestoreNeeded = OperatorDomain + "/restore-needed" // Mark PVC for restore

	// Operation parameters
	AnnotationManualBackupName  = OperatorDomain + "/manual-backup-name"  // Name for manual backup
	AnnotationManualRestoreName = OperatorDomain + "/manual-restore-name" // Name for manual restore

	// State tracking
	AnnotationDeletionState    = OperatorDomain + "/deletion-state"    // Track deletion progress
	AnnotationOriginalReplicas = OperatorDomain + "/original-replicas" // Store replica counts
)

// Timeouts and intervals
const (
	DefaultRequeueInterval   = 30 * time.Second // Default requeue on error
	DefaultReconcileInterval = 5 * time.Minute  // Default reconcile interval
	JobTTLSeconds            = 600              // TTL for completed jobs

	ResticInitTimeout    = 1 * time.Minute  // Repository init timeout
	ResticBackupTimeout  = 15 * time.Minute // Backup operation timeout
	ResticRestoreTimeout = 15 * time.Minute // Restore operation timeout
)

// Default schedules for operations
const (
	DefaultRetentionSchedule    = "0 3 * * *" // Daily at 3 AM
	DefaultMaintenanceSchedule  = "0 2 * * 0" // Weekly on Sunday at 2 AM
	DefaultVerificationSchedule = "0 1 * * *" // Daily at 1 AM
)

// Resource states
const (
	// Phase states
	PhaseUnknown   = ""
	PhasePending   = "Pending"
	PhaseRunning   = "Running"
	PhaseCompleted = "Completed"
	PhaseFailed    = "Failed"

	// Deletion states
	DeletionStateCleanup         = "cleanup"          // Resource is cleaning up dependencies
	DeletionStateRemoveFinalizer = "remove-finalizer" // Resource is ready for finalizer removal

	// Restore types
	RestoreTypeManual    = "manual"    // Manual restore operation
	RestoreTypeAutomated = "automated" // Automated restore operation
)
