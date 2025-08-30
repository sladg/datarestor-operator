/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BackupTarget defines a backup destination with priority
type BackupTarget struct {
	// Name of the backup target
	// +required
	Name string `json:"name"`

	// Priority determines the order of backup targets (lower number = higher priority)
	// +required
	Priority int32 `json:"priority"`

	// Restic configuration for backup targets
	// +required
	Restic *ResticConfig `json:"restic"`

	// Retention policy for this target
	// +optional
	Retention *RetentionPolicy `json:"retention,omitempty"`

	// NFS configuration for dynamic mounting (optional)
	// +optional
	NFS *NFSConfig `json:"nfs,omitempty"`
}

// NFSConfig defines NFS mount configuration for dynamic mounting
type NFSConfig struct {
	// NFS server address
	// +required
	Server string `json:"server"`

	// NFS export path
	// +required
	Path string `json:"path"`

	// NFS mount options
	// +optional
	MountOptions []string `json:"mountOptions,omitempty"`

	// Mount point in the container
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// RetentionPolicy defines how many snapshots to keep
type RetentionPolicy struct {
	// Maximum number of snapshots to keep
	// +required
	MaxSnapshots int32 `json:"maxSnapshots"`

	// Maximum age of snapshots to keep
	// +optional
	MaxAge *metav1.Duration `json:"maxAge,omitempty"`
}

// PVCSelector defines how to select PVCs for backup
type PVCSelector struct {
	// Label selector for PVCs
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Namespace selector
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// PVC names to include
	// +optional
	Names []string `json:"names,omitempty"`
}

// BackupSchedule defines when to take snapshots
type BackupSchedule struct {
	// Cron expression for backup schedule (e.g., "0 2 * * *" for daily at 2 AM)
	// +required
	// +kubebuilder:validation:Pattern=`^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$`
	Cron string `json:"cron"`

	// Whether to stop pods during backup for data integrity
	// +optional
	StopPods bool `json:"stopPods,omitempty"`

	// Wait for pod health before backup
	// +optional
	WaitForHealthy bool `json:"waitForHealthy,omitempty"`
}

// PVCBackupSpec defines the desired state of PVCBackup
type PVCBackupSpec struct {
	// PVC selector configuration
	// +required
	PVCSelector PVCSelector `json:"pvcSelector"`

	// Backup targets with priorities
	// +required
	BackupTargets []BackupTarget `json:"backupTargets"`

	// Backup schedule configuration
	// +required
	Schedule BackupSchedule `json:"schedule"`

	// Whether to enable automatic restore for new PVCs
	// +optional
	AutoRestore bool `json:"autoRestore,omitempty"`

	// Init container configuration for restore waiting
	// +optional
	InitContainer *InitContainerConfig `json:"initContainer,omitempty"`

	// Restic configuration for plain data backup
	// +optional
	Restic *ResticConfig `json:"restic,omitempty"`

	// Manual backup trigger. If set to true, a backup will be triggered immediately and the field will be reset to false after completion.
	// +optional
	ManualBackup bool `json:"manualBackup,omitempty"`
}

// InitContainerConfig defines init container for restore waiting
type InitContainerConfig struct {
	// Image for the init container
	// +required
	Image string `json:"image"`

	// Command for the init container
	// +optional
	Command []string `json:"command,omitempty"`

	// Args for the init container
	// +optional
	Args []string `json:"args,omitempty"`
}

// ResticConfig defines restic backup configuration
type ResticConfig struct {
	// Restic repository URL (e.g., s3:s3.amazonaws.com/bucket, nfs:/mnt/backup, sftp:user@host:/path)
	// +required
	Repository string `json:"repository"`

	// Password for restic repository
	// +optional
	Password string `json:"password,omitempty"`

	// Environment variables for authentication (e.g., AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Additional restic flags
	// +optional
	Flags []string `json:"flags,omitempty"`

	// Backup tags for organization
	// +optional
	Tags []string `json:"tags,omitempty"`

	// Hostname for backup identification
	// +optional
	Host string `json:"host,omitempty"`

	// Image for the restic job
	// +optional
	Image string `json:"image,omitempty"`
}

// PVCBackupStatus defines the observed state of PVCBackup.
type PVCBackupStatus struct {
	// List of PVCs being managed
	// +optional
	ManagedPVCs []string `json:"managedPVCs,omitempty"`

	// Last backup timestamp
	// +optional
	LastBackup *metav1.Time `json:"lastBackup,omitempty"`

	// Next backup timestamp
	// +optional
	NextBackup *metav1.Time `json:"nextBackup,omitempty"`

	// Last restore timestamp
	// +optional
	LastRestore *metav1.Time `json:"lastRestore,omitempty"`

	// Current backup status
	// +optional
	BackupStatus string `json:"backupStatus,omitempty"`

	// Current restore status
	// +optional
	RestoreStatus string `json:"restoreStatus,omitempty"`

	// Number of successful backups
	// +optional
	SuccessfulBackups int32 `json:"successfulBackups,omitempty"`

	// Number of failed backups
	// +optional
	FailedBackups int32 `json:"failedBackups,omitempty"`

	// Number of matching PVCs managed by this PVCBackup
	// +optional
	MatchingPVCs int32 `json:"matchingPVCs,omitempty"`

	// Number of backup targets configured
	// +optional
	BackupTargets int32 `json:"backupTargets,omitempty"`

	// Backup job statistics
	// +optional
	BackupJobs *JobStatistics `json:"backupJobs,omitempty"`

	// Restore job statistics
	// +optional
	RestoreJobs *JobStatistics `json:"restoreJobs,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// JobStatistics provides detailed statistics about backup/restore jobs
type JobStatistics struct {
	// Number of successful jobs
	// +optional
	Successful int32 `json:"successful,omitempty"`

	// Number of currently running jobs
	// +optional
	Running int32 `json:"running,omitempty"`

	// Number of failed jobs
	// +optional
	Failed int32 `json:"failed,omitempty"`

	// Number of pending jobs
	// +optional
	Pending int32 `json:"pending,omitempty"`

	// Total number of jobs
	// +optional
	Total int32 `json:"total,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PVCs",type="integer",JSONPath=".status.matchingPVCs"
// +kubebuilder:printcolumn:name="Targets",type="integer",JSONPath=".status.backupTargets"
// +kubebuilder:printcolumn:name="Backup Success",type="integer",JSONPath=".status.backupJobs.successful"
// +kubebuilder:printcolumn:name="Backup Running",type="integer",JSONPath=".status.backupJobs.running"
// +kubebuilder:printcolumn:name="Backup Failed",type="integer",JSONPath=".status.backupJobs.failed"
// +kubebuilder:printcolumn:name="Restore Success",type="integer",JSONPath=".status.restoreJobs.successful"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PVCBackup is the Schema for the pvcbackups API
type PVCBackup struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of PVCBackup
	// +required
	Spec PVCBackupSpec `json:"spec"`

	// status defines the observed state of PVCBackup
	// +optional
	Status PVCBackupStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// PVCBackupList contains a list of PVCBackup
type PVCBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PVCBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PVCBackup{}, &PVCBackupList{})
}
