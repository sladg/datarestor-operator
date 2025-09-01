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

// BackupTarget defines a backup destination with priority
type BackupTarget struct {
	// Name of the backup target
	// +required
	Name string `json:"name"`

	// Priority determines the order of backup targets (lower number = higher priority)
	// +required
	Priority int32 `json:"priority"`

	// Reference to the ResticRepository to use for backups
	// +required
	Repository ResticRepositoryRef `json:"repository"`

	// Cron schedule for retention checks (e.g., "0 3 * * *" for daily at 3 AM)
	// If not specified, defaults to "0 3 * * *"
	// +optional
	// +kubebuilder:default="0 3 * * *"
	// +kubebuilder:validation:Pattern=`^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$`
	RetentionSchedule string `json:"retentionSchedule,omitempty"`

	// Cron expression for backup schedule (e.g., "0 2 * * *" for daily at 2 AM)
	// +required
	// +kubebuilder:validation:Pattern=`^(\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])|\*\/([0-9]|1[0-9]|2[0-9]|3[0-9]|4[0-9]|5[0-9])) (\*|([1-9]|1[0-9]|2[0-9]|3[0-1])|\*\/([1-9]|1[0-9]|2[0-9]|3[0-1])) (\*|([1-9]|1[0-2])|\*\/([1-9]|1[0-2])) (\*|([0-6])|\*\/([0-6]))$`
	BackupSchedule string `json:"backupSchedule,omitempty"`
}

// Selector defines how to select PVCs for backup.
type Selector struct {
	// Label selector for PVCs.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

	// Annotation selector for PVCs. A PVC is selected if it has all the annotations in this map.
	// +optional
	AnnotationSelector map[string]string `json:"annotationSelector,omitempty"`

	// A list of namespaces to select PVCs from. If empty, it will search in all namespaces.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// A list of PVC names to select.
	// +optional
	Names []string `json:"names,omitempty"`

	// RepositoryName selects resources by the name of their associated ResticRepository.
	// This is only applicable for ResticBackup and ResticRestore types.
	// +optional
	RepositoryName string `json:"repositoryName,omitempty"`

	// StopPods specifies whether to scale down workloads using the PVCs matched by this selector.
	// +optional
	StopPods bool `json:"stopPods,omitempty"`

	// Wait for pod health before backup
	// +optional
	WaitForHealthy bool `json:"waitForHealthy,omitempty"`
}

// BackupConfigSpec defines the desired state of BackupConfig
type BackupConfigSpec struct {
	// A list of selectors for PVCs to be backed up. A PVC is selected if it matches any of the selectors in this list.
	// +required
	Selectors []Selector `json:"selectors"`

	// Backup targets with priorities
	// +required
	BackupTargets []BackupTarget `json:"backupTargets"`

	// Whether to enable automatic restore for new PVCs
	// +optional
	AutoRestore bool `json:"autoRestore,omitempty"`
}

// BackupConfigStatus defines the observed state of BackupConfig
type BackupConfigStatus struct {
	// Embed common status fields
	CommonStatus `json:",inline"`

	// Number of PVCs currently selected by this config
	// +optional
	PVCsCount int32 `json:"pvcsCount,omitempty"`

	// Number of backup targets
	// +optional
	TargetsCount int32 `json:"targetsCount,omitempty"`

	// List of repositories managed by this config
	// +optional
	Repositories []ResticRepositoryRef `json:"repositories,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="PVCs",type="integer",JSONPath=".status.pvcsCount"
// +kubebuilder:printcolumn:name="Targets",type="integer",JSONPath=".status.targetsCount"
// +kubebuilder:printcolumn:name="Auto-Restore",type="boolean",JSONPath=".spec.autoRestore"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// BackupConfig is the Schema for the backupconfigs API
type BackupConfig struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the desired state of BackupConfig
	// +required
	Spec BackupConfigSpec `json:"spec"`

	// status contains the observed state
	// +optional
	Status BackupConfigStatus `json:"status,omitempty"`
}

// GetLogValues returns key-value pairs for structured logging.
func (b *BackupConfig) GetLogValues() []interface{} {
	return []interface{}{
		"config", b.Name,
		"namespace", b.Namespace,
	}
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
