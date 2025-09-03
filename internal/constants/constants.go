package constants

import "time"

// Domain for the operator
const OperatorDomain = "backup.datarestor-operator.com"

// Labels used for resource identification
const (
	// Resource identifiers
	LabelPVCName      = OperatorDomain + "/pvc-name"      // Name of related PVC
	LabelBackupConfig = OperatorDomain + "/backup-config" // Name of related BackupConfig
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

	// State tracking
	AnnotationDeletionState    = OperatorDomain + "/deletion-state"     // Track deletion progress
	AnnotationOriginalReplicas = OperatorDomain + "/original-replicas"  // Store replica counts
	AnnotationDeleteResticData = OperatorDomain + "/delete-restic-data" // Whether to delete restic data on backup deletion
)

// Timeouts and intervals
const (
	DefaultRequeueInterval = 30 * time.Second // Default requeue on error
	// Additional requeue intervals
	FailedRequeueInterval     = 5 * time.Minute // Requeue interval after a failed operation
	RepositoryRequeueInterval = 1 * time.Hour   // Requeue interval for repository maintenance or long waits
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
