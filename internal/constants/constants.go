package constants

import "time"

// Constants for annotations, labels, finalizers, and operational settings used throughout the operator

const (
	// Domain for the operator
	OperatorDomain = "backup.autorestore-backup-operator.com"

	// Labels
	PVCRestoreLabel = "backup.restore"                                 // Label to mark PVCs for automatic restore
	LabelPVC        = "backupconfig." + OperatorDomain + "/pvc"        // Label for PVC name
	LabelPVCBackup  = "backupconfig." + OperatorDomain + "/pvcbackup"  // Label for BackupConfig name
	LabelCreatedBy  = "backupconfig." + OperatorDomain + "/created-by" // Label for resource creator
	LabelResticJob  = "backupconfig." + OperatorDomain + "/restic-job" // Label for restic job type
	LabelPVCName    = "backupconfig." + OperatorDomain + "/pvc-name"   // Label for PVC name in restic jobs
	LabelJobName    = "job-name"                                       // Standard Kubernetes job-name label

	// Annotations
	OriginalReplicasAnnotation     = OperatorDomain + "/original-replicas"           // Stores original replica count during scaling
	AnnotationManualBackupTrigger  = OperatorDomain + "/manual-backup-trigger"       // Triggers manual backup when set to "now"
	AnnotationManualRestoreTrigger = OperatorDomain + "/manual-restore-trigger"      // Triggers manual restore on PVC when set to "now"
	AnnotationBackupID             = "backupconfig." + OperatorDomain + "/backup-id" // Stores backup ID
	AnnotationRestoreNeeded        = "restore." + OperatorDomain + "/needed"         // Marks PVC as needing restore

	// Finalizers
	BackupConfigFinalizer = "backupconfig." + OperatorDomain + "/finalizer" // BackupConfig finalizer

	ResticRestoreFinalizer    = "resticrestore." + OperatorDomain + "/finalizer"           // ResticRestore finalizer
	ResticRepositoryFinalizer = "resticrepository." + OperatorDomain + "/finalizer"        // ResticRepository finalizer
	ResticBackupFinalizer     = "resticbackup." + OperatorDomain + "/finalizer"            // ResticBackup finalizer
	PVCRestoreFinalizer       = "resticrestore." + OperatorDomain + "/restore-in-progress" // PVC finalizer during automated restore
	PVCBackupFinalizer        = "resticbackup." + OperatorDomain + "/backup-in-progress"   // PVC finalizer during backup operations

	// Timing and operational constants
	DefaultRequeueInterval   = 30 * time.Second // Default requeue interval for errors
	DefaultReconcileInterval = 5 * time.Minute  // Default reconcile interval for BackupConfig
	JobTTLSeconds            = 600              // TTL for completed Kubernetes Jobs

	// Restic job timeouts
	ResticInitTimeout    = 1 * time.Minute  // Timeout for repository initialization jobs
	ResticBackupTimeout  = 15 * time.Minute // Timeout for backup jobs
	ResticRestoreTimeout = 15 * time.Minute // Timeout for restore jobs

	// Default Restic Password
	DefaultResticPassword = "default-password" // TODO: Move to proper secret handling
)

// Job phase constants
type JobPhase string

const (
	JobPhasePending   JobPhase = "Pending"
	JobPhaseRunning   JobPhase = "Running"
	JobPhaseCompleted JobPhase = "Completed"
	JobPhaseFailed    JobPhase = "Failed"
)

// RestoreType constants (for spec)
type RestoreType string

const (
	RestoreTypeManual    RestoreType = "manual"
	RestoreTypeAutomated RestoreType = "automated"
)

// Cleanup state constants
type CleanupState string

const (
	// Cleanup state values
	CleanupStateIdle       CleanupState = "idle"
	CleanupStateInProgress CleanupState = "in-progress"
	CleanupStateCompleted  CleanupState = "completed"

	// Annotations for backup cleanup
	AnnotationCleanupState     = OperatorDomain + "/cleanup-state"         // Current state of cleanup process
	AnnotationCurrentTargetIdx = OperatorDomain + "/current-target-index"  // Index of target being processed
	AnnotationCurrentPVCName   = OperatorDomain + "/current-pvc-name"      // Name of PVC being processed
	AnnotationCurrentPVCNS     = OperatorDomain + "/current-pvc-namespace" // Namespace of PVC being processed
	AnnotationLastCleanupTime  = OperatorDomain + "/last-cleanup-time"     // When the last cleanup completed

)

// Deletion state constants for single-action-per-loop pattern in finalizers
type DeletionState string

const (
	// Deletion state values for RestoreJob controller
	DeletionStateCleanup         DeletionState = "cleanup"
	DeletionStateRemoveFinalizer DeletionState = "remove-finalizer"

	// Annotations for deletion state tracking
	AnnotationDeletionState = OperatorDomain + "/deletion-state" // Current state of deletion process

	// Default cron schedules
	DefaultRetentionSchedule    = "0 3 * * *" // Daily at 3 AM
	DefaultMaintenanceSchedule  = "0 2 * * 0" // Weekly on Sunday at 2 AM
	DefaultVerificationSchedule = "0 1 * * *" // Daily at 1 AM
)
