package constants

import (
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
)

const (
	LabelControllerUID = "controller-uid"
)

const (
	BackupConfigFinalizer     = v1.OperatorDomain + "/backupconfig-finalizer"     // BackupConfig finalizer
	ResticRepositoryFinalizer = v1.OperatorDomain + "/resticrepository-finalizer" // ResticRepository finalizer
	ResticBackupFinalizer     = v1.OperatorDomain + "/resticbackup-finalizer"     // ResticBackup finalizer
	ResticRestoreFinalizer    = v1.OperatorDomain + "/resticrestore-finalizer"    // ResticRestore finalizer
)

const (
	// Trigger manual backup with optional snapshot ID (e.g., `now`, `latest`, or specific ID)
	AnnotationManualBackup = v1.OperatorDomain + "/manual-backup"

	// Trigger manual restore. It allows for annotation such as `latest`, `now`, or specific snapshot IDs.
	// It also allows for specifying `pvcName#snapshotID` to restore from other PVC snapshots. Use "pvcName#now".
	AnnotationManualRestore = v1.OperatorDomain + "/manual-restore"

	AnnotationOriginalReplicas = v1.OperatorDomain + "/original-replicas" // Store replica counts

	// Backup args, such as --exclude patterns
	AnnotationBackupArgs = v1.OperatorDomain + "/backup-args"

	// Restore args, such as --include patterns
	AnnotationRestoreArgs = v1.OperatorDomain + "/restore-args"
)

const (
	QuickRequeueInterval     = 5 * time.Second
	DefaultRequeueInterval   = 30 * time.Second
	ImmediateRequeueInterval = 1 * time.Second
	LongerRequeueInterval    = 120 * time.Second
	VeryLongRequeueInterval  = 300 * time.Second
)

var ActivePhases = []v1.Phase{v1.PhaseRunning, v1.PhasePending}
