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

// ResticRestoreSpec defines the desired state of ResticRestore
type ResticRestoreSpec struct {
	// Reference to the ResticRepository in the same namespace
	// +required
	Repository ResticRepositoryRef `json:"repository"`

	// Name of the ResticBackup to restore from
	// +required
	Name string `json:"name"`

	// +required
	Type RestoreType `json:"type,omitempty"`

	// Target PVC name to restore data into
	// +required
	TargetPVC PersistentVolumeClaimRef `json:"targetPVC"`

	// Restic configuration for restore target
	// +required
	Restic ResticRepositorySpec `json:"restic"`

	// Arguments to pass to restic restore command
	// Examples: ["--path", "/data", "--include", "*.sql", "--exclude", "cache/"]
	// +optional
	Args []string `json:"args,omitempty"`

	// Specific snapshot ID to restore from (optional, uses latest if not specified)
	// +optional
	SnapshotID string `json:"snapshotID,omitempty"`
}

// ResticRestoreStatus defines the observed state of ResticRestore
type ResticRestoreStatus struct {
	// Embed common status fields
	CommonStatus `json:",inline"`

	// Reference to the restore job
	// +optional
	Job JobReference `json:"job,omitempty"`

	// Duration of the restore operation
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`

	// Time when the backup was created in the repository
	// +optional
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Backup",type="string",JSONPath=".spec.name"
//+kubebuilder:printcolumn:name="Target PVC",type="string",JSONPath=".spec.targetPVC.name"
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
//+kubebuilder:printcolumn:name="Duration",type="string",JSONPath=".status.duration"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticRestore is the Schema for the resticrestores API
type ResticRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticRestoreSpec   `json:"spec,omitempty"`
	Status ResticRestoreStatus `json:"status,omitempty"`
}

// GetLogValues returns key-value pairs for structured logging.
func (r *ResticRestore) GetLogValues() []interface{} {
	return []interface{}{
		"restore", r.Name,
		"namespace", r.Namespace,
		"phase", r.Status.Phase,
	}
}

//+kubebuilder:object:root=true

// ResticRestoreList contains a list of ResticRestore
type ResticRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResticRestore `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ResticRestore{}, &ResticRestoreList{})
}
