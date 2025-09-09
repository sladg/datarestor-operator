package constants

import (
	"fmt"
	"runtime/debug"
	"time"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"

	// Keep restic dependency for version detection
	_ "github.com/restic/restic"
)

const (
	LabelControllerUID = "controller-uid"
)

const (
	ConfigFinalizer = v1.OperatorDomain + "/config-finalizer" // Config finalizer
	TaskFinalizer   = v1.OperatorDomain + "/task-finalizer"   // Task finalizer
)

const (
	// Trigger manual backup with optional snapshot ID (e.g., `now`, `latest`, or specific ID)
	AnnBackup = v1.OperatorDomain + "/manual-backup"

	// Used to specify exact backupName/pvcName/repository to restore from
	AnnRestore = v1.OperatorDomain + "/auto-restore"

	// Store replica counts
	AnnOriginalReplicas = v1.OperatorDomain + "/original-replicas"

	// Backup/Restore args, such as --exclude patterns
	AnnArgs = v1.OperatorDomain + "/backup-args"

	// Track and avoid duplicated auto-restore processes
	AnnAutoRestored = v1.OperatorDomain + "/auto-restore-processed"

	LabelTaskParentName      = v1.OperatorDomain + "/task-parent-name"
	LabelTaskParentNamespace = v1.OperatorDomain + "/task-parent-namespace"
)

const (
	QuickRequeueInterval     = 5 * time.Second
	DefaultRequeueInterval   = 30 * time.Second
	ImmediateRequeueInterval = 2 * time.Second
	LongerRequeueInterval    = 120 * time.Second
	VeryLongRequeueInterval  = 300 * time.Second
	MaxAgeForNewPVC          = 15 * time.Minute
)

var ActivePhases = []v1.Phase{v1.PhaseRunning, v1.PhasePending}

// GetResticImage returns the full restic image string with version from dependency
func GetResticImage() string {
	info, ok := debug.ReadBuildInfo()

	if !ok {
		return "restic/restic:latest"
	}

	version := "latest"
	for _, dep := range info.Deps {
		if dep.Path == "github.com/restic/restic" {
			version = dep.Version
		}
	}

	if version == "latest" {
		return "restic/restic:latest"
	}

	// Convert module version (e.g., v0.18.0) to image tag (e.g., 0.18.0)
	if len(version) > 1 && version[0] == 'v' {
		version = version[1:]
	}

	return fmt.Sprintf("restic/restic:%s", version)
}
