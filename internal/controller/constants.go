package controller

// Constants for annotations, labels, and finalizers used throughout the operator

const (
	// Domain for the operator
	OperatorDomain = "backup.autorestore-backup-operator.com"

	// Labels
	PVCRestoreLabel = "backup.restore" // Label to mark PVCs for automatic restore

	// Annotations
	OriginalReplicasAnnotation = OperatorDomain + "/original-replicas" // Stores original replica count during scaling

	// Finalizers
	BackupConfigFinalizer = "backupconfig." + OperatorDomain + "/finalizer"         // BackupConfig finalizer
	BackupJobFinalizer    = "backupjob." + OperatorDomain + "/finalizer"            // BackupJob finalizer
	RestoreJobFinalizer   = "restorejob." + OperatorDomain + "/finalizer"           // RestoreJob finalizer
	PVCRestoreFinalizer   = "restorejob." + OperatorDomain + "/restore-in-progress" // PVC finalizer during automated restore
)
