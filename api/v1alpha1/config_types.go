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

// Selector defines how to select PVCs for backup.
type Selector struct {
	// Label selector for PVCs.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// A list of namespaces to select PVCs from. If empty, it will search in all namespaces.
	// +optional
	MatchNamespaces []string `json:"matchNamespaces,omitempty"`

	// StopPods specifies whether to scale down workloads using the PVCs matched by this selector.
	// +optional
	// +kubebuilder:default=false
	StopPods bool `json:"stopPods,omitempty"`

	// AutoRestore specifies whether to enable automatic restore for new PVCs
	// +optional
	// +kubebuilder:default=true
	AutoRestore bool `json:"autoRestore,omitempty"`
}

type ConfigStatistics struct {
	// Number of backups run successfully so far
	// +optional
	SuccessfulBackups int32 `json:"successfulBackups,omitempty"`

	// Number of backups run failed so far
	// +optional
	FailedBackups int32 `json:"failedBackups,omitempty"`

	// Number of backups in-flight currently
	// +optional
	RunningBackups int32 `json:"runningBackups,omitempty"`
}

type RepositorySpec struct {
	// Restic repository target (e.g., s3:s3.amazonaws.com/bucket/path)
	// +required
	Target string `json:"target"`

	// Priority determines the order of backup targets (lower number = higher priority)
	// +required
	Priority int32 `json:"priority"`

	// Environment variables for authentication (e.g., AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Cron expression for backup schedule (e.g., "0 2 * * *" for daily at 2 AM)
	// +required
	// +kubebuilder:validation:Pattern=`^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$`
	// +kubebuilder:default="0 2 * * *"
	BackupSchedule string `json:"backupSchedule,omitempty"`

	// @TODO: Add retention schedule

	// A list of selectors for PVCs to be backed up. A PVC is selected if it matches any of the selectors in this list.
	// +optional
	Selectors []Selector `json:"selectors,omitempty"`

	// Internal status of the repository
	// +optional
	Status RepositoryStatus `json:"-"`
}

type RepositoryStatus struct {
	// Restic repository target (e.g., s3:s3.amazonaws.com/bucket/path)
	// +optional
	Target string `json:"target"` // @TODO: Remove, it's on Spec

	// Time when the repository was initialized
	// +optional
	InitializedAt *metav1.Time `json:"initializedAt,omitempty"`

	// Known backups
	// +optional
	Backups []string `json:"backups,omitempty"`

	// Information about the last scheduled backup run for this target
	// +optional
	LastScheduledBackupRun *metav1.Time `json:"lastScheduledBackupRun,omitempty"`
}

type WorkloadInfo struct {
	// Kind of the workload
	// +required
	Kind string `json:"kind"`

	// Name of the workload
	// +required
	Name string `json:"name"`

	// Replicas of the workload
	// +required
	Replicas int32 `json:"replicas"`
}

// ConfigStatus defines the observed state of Config
type ConfigStatus struct {
	// Number of PVCs currently selected by this config
	// +optional
	MatchedPVCsCount int32 `json:"matchedPVCsCount,omitempty"`

	// Statistics about the repository
	// +optional
	Statistics ConfigStatistics `json:"statistics,omitempty"`

	// @TODO: Add LastMaintenanceRun

	// Workload information
	// +optional
	WorkloadInfo map[string]WorkloadInfo `json:"workloadInfo,omitempty"`

	// Repositories status
	// +optional
	Repositories []RepositorySpec `json:"repositories,omitempty"`

	// Initialized at
	// +optional
	InitializedAt *metav1.Time `json:"initializedAt,omitempty"`
}

type ConfigSpec struct {
	// A list of repositories to be backed up. A repository is selected if it matches any of the repositories in this list.
	// +required
	// +minItems=1
	Repositories []RepositorySpec `json:"repositories,omitempty"`

	// Environment variables for authentication (e.g., AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// A list of selectors for PVCs to be backed up. A PVC is selected if it matches any of the selectors in this list.
	// +optional
	Selectors []Selector `json:"selectors,omitempty"`

	// Whether to enable automatic restore for new PVCs
	// +optional
	// +kubebuilder:default=true
	AutoRestore bool `json:"autoRestore,omitempty"`

	// StopPods specifies whether to scale down workloads using the PVCs matched by this selector.
	// +optional
	// +kubebuilder:default=false
	StopPods bool `json:"stopPods,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PVCs",type="integer",JSONPath=".status.matchedPVCsCount"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Config is the Schema for the configs API
type Config struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of Config
	// +required
	Spec ConfigSpec `json:"spec"`

	// status contains the observed state
	// +optional
	Status ConfigStatus `json:"status,omitempty"`
}

// GetLogValues returns key-value pairs for structured logging.
func (b *Config) GetLogValues() []interface{} {
	return []interface{}{
		"config", b.Name,
		"namespace", b.Namespace,
	}
}

// +kubebuilder:object:root=true

// BackupConfigList contains a list of BackupConfig
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}
