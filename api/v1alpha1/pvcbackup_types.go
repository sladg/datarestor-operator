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

	// Type of backup target (s3, nfs, etc.)
	// +required
	Type string `json:"type"`

	// S3 configuration for S3-type targets
	// +optional
	S3 *S3Config `json:"s3,omitempty"`

	// NFS configuration for NFS-type targets
	// +optional
	NFS *NFSConfig `json:"nfs,omitempty"`

	// Retention policy for this target
	// +optional
	Retention *RetentionPolicy `json:"retention,omitempty"`
}

// S3Config defines S3 backup target configuration
type S3Config struct {
	// S3 bucket name
	// +required
	Bucket string `json:"bucket"`

	// S3 region
	// +required
	Region string `json:"region"`

	// S3 endpoint (for custom S3-compatible storage)
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// S3 access key ID
	// +required
	AccessKeyID string `json:"accessKeyID"`

	// S3 secret access key
	// +required
	SecretAccessKey string `json:"secretAccessKey"`

	// S3 path prefix
	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`
}

// NFSConfig defines NFS backup target configuration
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
	// Cron expression for backup schedule
	// +required
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
	// Restic repository
	// +required
	Repository string `json:"repository"`

	// Restic password
	// +required
	Password string `json:"password"`

	// Additional restic flags
	// +optional
	Flags []string `json:"flags,omitempty"`
}

// PVCBackupStatus defines the observed state of PVCBackup.
type PVCBackupStatus struct {
	// List of PVCs being managed
	// +optional
	ManagedPVCs []string `json:"managedPVCs,omitempty"`

	// Last backup timestamp
	// +optional
	LastBackup *metav1.Time `json:"lastBackup,omitempty"`

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

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.backupStatus"
// +kubebuilder:printcolumn:name="Last Backup",type="date",JSONPath=".status.lastBackup"
// +kubebuilder:printcolumn:name="PVCs",type="integer",JSONPath=".status.managedPVCs"
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
