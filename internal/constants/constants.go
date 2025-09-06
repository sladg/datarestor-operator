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
	AnnotationManualBackup  = v1.OperatorDomain + "/manual-backup"  // Trigger manual backup
	AnnotationManualRestore = v1.OperatorDomain + "/manual-restore" // Trigger manual restore

	AnnotationOriginalReplicas = v1.OperatorDomain + "/original-replicas" // Store replica counts
)

const (
	QuickRequeueInterval     = 5 * time.Second
	DefaultRequeueInterval   = 30 * time.Second
	ImmediateRequeueInterval = 1 * time.Second
	LongerRequeueInterval    = 120 * time.Second
)

var ActivePhases = []v1.Phase{v1.PhaseRunning, v1.PhasePending}
