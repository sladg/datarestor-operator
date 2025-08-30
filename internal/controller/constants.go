package controller

// Constants for annotations, labels, and finalizers used throughout the operator

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

	// Annotations
	OriginalReplicasAnnotation = OperatorDomain + "/original-replicas"           // Stores original replica count during scaling
	AnnotationManualTrigger    = OperatorDomain + "/manual-trigger"              // Triggers manual backup when set to "now"
	AnnotationBackupID         = "backupconfig." + OperatorDomain + "/backup-id" // Stores backup ID
	AnnotationRestoreNeeded    = "restore." + OperatorDomain + "/needed"         // Marks PVC as needing restore

	// Finalizers
	BackupConfigFinalizer = "backupconfig." + OperatorDomain + "/finalizer"         // BackupConfig finalizer
	BackupJobFinalizer    = "backupjob." + OperatorDomain + "/finalizer"            // BackupJob finalizer
	RestoreJobFinalizer   = "restorejob." + OperatorDomain + "/finalizer"           // RestoreJob finalizer
	PVCRestoreFinalizer   = "restorejob." + OperatorDomain + "/restore-in-progress" // PVC finalizer during automated restore
)
