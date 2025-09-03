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

// ResticBackupSpec defines the desired state of ResticBackup
type ResticBackupSpec struct {
	// Reference to the ResticRepository in the same namespace
	// +required
	Repository corev1.ObjectReference `json:"repository"`

	// Name of the backup
	// +required
	Name string `json:"name"`

	// Type of backup (scheduled, manual)
	// +optional
	// +kubebuilder:default="scheduled"
	// +kubebuilder:validation:Enum=scheduled;manual;auto-restore
	Type BackupType `json:"type,omitempty"`

	// PVC that was/is being backed up
	// +required
	SourcePVC corev1.ObjectReference `json:"sourcePVC"`

	// Arguments to pass to restic restore command
	// Examples: ["--path", "/data", "--include", "*.sql", "--exclude", "cache/"]
	// +optional
	Args []string `json:"args,omitempty"`

	// Specific snapshot ID to restore from (optional, uses latest if not specified)
	// +optional
	SnapshotID string `json:"snapshotID,omitempty"`
}

// ResticBackupStatus defines the observed state of ResticBackup (merged with BackupJob)
type ResticBackupStatus struct {
	// Embed common status fields
	CommonStatus `json:",inline"`

	// Reference to the backup job
	// +optional
	Job corev1.ObjectReference `json:"job,omitempty"`

	// Duration of the backup job as a string.
	// +optional
	Duration metav1.Duration `json:"duration,omitempty"`

	// Time when the backup was created in the repository
	// +optional
	CreatedAt metav1.Time `json:"createdAt,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Backup Name",type="string",JSONPath=".spec.name"
// +kubebuilder:printcolumn:name="PVC",type="string",JSONPath=".spec.sourcePVC.name"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="Duration",type="string",JSONPath=".status.duration"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticBackup is the Schema for the resticbackups API
type ResticBackup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticBackupSpec   `json:"spec,omitempty"`
	Status ResticBackupStatus `json:"status,omitempty"`
}

// GetLogValues returns key-value pairs for structured logging.
func (r *ResticBackup) GetLogValues() []interface{} {
	return []interface{}{
		"backup", r.Name,
		"namespace", r.Namespace,
		"phase", r.Status.Phase,
	}
}

// +kubebuilder:object:root=true

// ResticBackupList contains a list of ResticBackup
type ResticBackupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResticBackup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResticBackup{}, &ResticBackupList{})
}
