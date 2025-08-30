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

// PVCBackupRestoreJobSpec defines the desired state of PVCBackupRestoreJob
type PVCBackupRestoreJobSpec struct {
	// Reference to the PVC to restore data into
	// +required
	PVCRef corev1.LocalObjectReference `json:"pvcRef"`

	// Reference to the backup source (either PVCBackupJob or manual backup ID)
	// +optional
	BackupJobRef *corev1.LocalObjectReference `json:"backupJobRef,omitempty"`

	// Manual backup ID to restore from (alternative to BackupJobRef)
	// +optional
	BackupID string `json:"backupID,omitempty"`

	// Backup target configuration for restore
	// +required
	BackupTarget BackupTarget `json:"backupTarget"`

	// Reference to the parent PVCBackup resource
	// +optional
	PVCBackupRef corev1.LocalObjectReference `json:"pvcBackupRef,omitempty"`

	// Type of restore operation (manual, automated)
	// +required
	RestoreType string `json:"restoreType"`

	// Whether to overwrite existing data in the PVC
	// +optional
	// +kubebuilder:default=false
	OverwriteExisting bool `json:"overwriteExisting,omitempty"`
}

// PVCBackupRestoreJobStatus defines the observed state of PVCBackupRestoreJob
type PVCBackupRestoreJobStatus struct {
	// Current phase of the restore job (Pending, Running, Completed, Failed)
	// +optional
	Phase string `json:"phase,omitempty"`

	// Type of restore operation status (Manual, Automated, NotFound)
	// Manual: User explicitly requested restore
	// Automated: Auto-restore triggered by system
	// NotFound: No backup available for restore (normal for new PVCs)
	// +optional
	RestoreStatus string `json:"restoreStatus,omitempty"`

	// Time when the restore started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Time when the restore completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Backup ID that was restored
	// +optional
	RestoredBackupID string `json:"restoredBackupID,omitempty"`

	// Amount of data restored (in bytes)
	// +optional
	DataRestored int64 `json:"dataRestored,omitempty"`

	// Repository URL used for restore
	// +optional
	Repository string `json:"repository,omitempty"`

	// Error message if restore failed
	// +optional
	Error string `json:"error,omitempty"`

	// Reference to the Kubernetes Job performing the restore
	// +optional
	JobRef *corev1.LocalObjectReference `json:"jobRef,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Restore Status",type="string",JSONPath=".status.restoreStatus"
// +kubebuilder:printcolumn:name="PVC",type="string",JSONPath=".spec.pvcRef.name"
// +kubebuilder:printcolumn:name="Target",type="string",JSONPath=".spec.backupTarget.name"
// +kubebuilder:printcolumn:name="Backup ID",type="string",JSONPath=".status.restoredBackupID"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// PVCBackupRestoreJob is the Schema for the pvcbackuprestorejobs API
type PVCBackupRestoreJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PVCBackupRestoreJobSpec   `json:"spec,omitempty"`
	Status PVCBackupRestoreJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// PVCBackupRestoreJobList contains a list of PVCBackupRestoreJob
type PVCBackupRestoreJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PVCBackupRestoreJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PVCBackupRestoreJob{}, &PVCBackupRestoreJobList{})
}
