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
}

// RetentionPolicy defines retention policy for snapshots using restic forget options
type RetentionPolicy struct {
	// Keep the last n snapshots (restic --keep-last)
	// +optional
	KeepLast *int32 `json:"keepLast,omitempty"`

	// Keep the most recent snapshot for each of the last n hours (restic --keep-hourly)
	// +optional
	KeepHourly *int32 `json:"keepHourly,omitempty"`

	// Keep the most recent snapshot for each of the last n days (restic --keep-daily)
	// +optional
	KeepDaily *int32 `json:"keepDaily,omitempty"`

	// Keep the most recent snapshot for each of the last n weeks (restic --keep-weekly)
	// +optional
	KeepWeekly *int32 `json:"keepWeekly,omitempty"`

	// Keep the most recent snapshot for each of the last n months (restic --keep-monthly)
	// +optional
	KeepMonthly *int32 `json:"keepMonthly,omitempty"`

	// Keep the most recent snapshot for each of the last n years (restic --keep-yearly)
	// +optional
	KeepYearly *int32 `json:"keepYearly,omitempty"`

	// Keep all snapshots within this duration (restic --keep-within, e.g. "2y5m7d3h")
	// +optional
	KeepWithin *metav1.Duration `json:"keepWithin,omitempty"`

	// Keep snapshots with these tags (restic --keep-tag)
	// +optional
	KeepTags []string `json:"keepTags,omitempty"`

	// Whether to run prune automatically after forget (restic --prune)
	// +optional
	// +kubebuilder:default=true
	Prune bool `json:"prune,omitempty"`

	// Cron schedule for retention checks (e.g., "0 3 * * *" for daily at 3 AM)
	// If not specified, defaults to "0 3 * * *"
	// +optional
	// +kubebuilder:default="0 3 * * *"
	// +kubebuilder:validation:Pattern=`^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$`
	Schedule string `json:"schedule,omitempty"`

	// Deprecated: Use KeepLast instead
	// +optional
	MaxSnapshots int32 `json:"maxSnapshots,omitempty"`
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

// BackupConfigSpec defines the desired state of BackupConfig
type BackupConfigSpec struct {
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

	// Manual backup trigger. If set to true, a backup will be triggered immediately and the field will be reset to false after completion.
	// +optional
	ManualBackup bool `json:"manualBackup,omitempty"`
}

// ResticConfig defines restic backup configuration
type ResticConfig struct {
	// Restic repository URL (e.g., s3:s3.amazonaws.com/bucket)
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

// BackupConfigStatus defines the observed state of BackupConfig.
type BackupConfigStatus struct {
	// List of PVCs being managed
	// +optional
	ManagedPVCs []string `json:"managedPVCs,omitempty"`

	// Last backup timestamp
	// +optional
	LastBackup *metav1.Time `json:"lastBackup,omitempty"`

	// Next backup timestamp
	// +optional
	NextBackup *metav1.Time `json:"nextBackup,omitempty"`

	// Backup job statistics
	// +optional
	BackupJobs *JobStatistics `json:"backupJobs,omitempty"`

	// Restore job statistics
	// +optional
	RestoreJobs *JobStatistics `json:"restoreJobs,omitempty"`

	// Last time retention policy was applied
	// +optional
	LastRetentionCheck *metav1.Time `json:"lastRetentionCheck,omitempty"`

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

// BackupConfig is the Schema for the backupconfigs API
type BackupConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty,omitzero"`

	// spec defines the desired state of BackupConfig
	// +required
	Spec BackupConfigSpec `json:"spec"`

	// status defines the observed state of BackupConfig
	// +optional
	Status BackupConfigStatus `json:"status,omitempty,omitzero"`
}

// +kubebuilder:object:root=true

// BackupConfigList contains a list of BackupConfig
type BackupConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BackupConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BackupConfig{}, &BackupConfigList{})
}
