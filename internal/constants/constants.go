package constants

import (
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
)

const (
	LabelPVCName      = v1.OperatorDomain + "/pvc-name"      // Name of related PVC
	LabelBackupConfig = v1.OperatorDomain + "/backup-config" // Name of related BackupConfig
)

const (
	BackupConfigFinalizer     = v1.OperatorDomain + "/backupconfig-finalizer"     // BackupConfig finalizer
	ResticRepositoryFinalizer = v1.OperatorDomain + "/resticrepository-finalizer" // ResticRepository finalizer
	ResticBackupFinalizer     = v1.OperatorDomain + "/resticbackup-finalizer"     // ResticBackup finalizer
	ResticRestoreFinalizer    = v1.OperatorDomain + "/resticrestore-finalizer"    // ResticRestore finalizer
	PVCRestoreFinalizer       = v1.OperatorDomain + "/pvc-restore-in-progress"    // PVC finalizer during restore
	WorkloadFinalizer         = v1.OperatorDomain + "/workload-finalizer"         // Workload finalizer
)

const (
	AnnotationManualBackup  = v1.OperatorDomain + "/manual-backup"  // Trigger manual backup
	AnnotationManualRestore = v1.OperatorDomain + "/manual-restore" // Trigger manual restore

	AnnotationOriginalReplicas = v1.OperatorDomain + "/original-replicas"  // Store replica counts
	AnnotationDeleteResticData = v1.OperatorDomain + "/delete-restic-data" // Whether to delete restic data on backup deletion
)

const (
	DefaultRequeueInterval    = 30 * time.Second
	FailedRequeueInterval     = 5 * time.Minute
	RepositoryRequeueInterval = 1 * time.Hour
	ImmediateRequeueInterval  = 0 * time.Second
)
