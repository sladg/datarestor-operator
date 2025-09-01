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

// ResticRestoreSpec defines the desired state of ResticRestore
type ResticRestoreSpec struct {
	// Name of the ResticBackup to restore from
	// +required
	BackupName string `json:"backupName"`

	// Target PVC name to restore data into
	// +required
	TargetPVC string `json:"targetPVC"`

	// Whether to create the target PVC if it doesn't exist
	// +optional
	// +kubebuilder:default=false
	CreateTargetPVC bool `json:"createTargetPVC,omitempty"`

	// Template for creating the target PVC (used when CreateTargetPVC is true)
	// +optional
	TargetPVCTemplate *corev1.PersistentVolumeClaim `json:"targetPVCTemplate,omitempty"`

	// Backup target configuration for restore
	// +required
	BackupTarget BackupTarget `json:"backupTarget"`

	// Whether to overwrite existing data in the PVC
	// +optional
	// +kubebuilder:default=false
	Overwrite bool `json:"overwrite,omitempty"`

	// Paths to restore (if empty, restore everything)
	// +optional
	Paths []string `json:"paths,omitempty"`
}

// ResticRestoreStatus defines the observed state of ResticRestore
type ResticRestoreStatus struct {
	// Current phase of the restore (Pending, Running, Completed, Failed)
	// +optional
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	Phase string `json:"phase,omitempty"`

	// Time when the restore started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Time when the restore completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Reference to the underlying Kubernetes Job
	// +optional
	JobRef *corev1.LocalObjectReference `json:"jobRef,omitempty"`

	// Error message if the restore failed
	// +optional
	Error string `json:"error,omitempty"`

	// Conditions represent the latest available observations of restore state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Amount of data restored in bytes
	// +optional
	RestoredSize int64 `json:"restoredSize,omitempty"`

	// Duration of the restore operation
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Backup",type="string",JSONPath=".spec.backupName"
//+kubebuilder:printcolumn:name="Target PVC",type="string",JSONPath=".spec.targetPVC"
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Duration",type="string",JSONPath=".status.duration"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ResticRestore is the Schema for the resticrestores API
type ResticRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ResticRestoreSpec   `json:"spec,omitempty"`
	Status ResticRestoreStatus `json:"status,omitempty"`
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
