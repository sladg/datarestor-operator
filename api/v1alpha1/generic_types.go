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

type Phase string

const (
	PhaseUnknown   Phase = ""
	PhasePending   Phase = "Pending"
	PhaseRunning   Phase = "Running"
	PhaseCompleted Phase = "Completed"
	PhaseFailed    Phase = "Failed"
	PhaseDeletion  Phase = "Deletion"
)

type BackupType string

const (
	BackupTypeScheduled BackupType = "scheduled"
	BackupTypeManual    BackupType = "manual"
)

type RestoreType string

const (
	RestoreTypeManual    RestoreType = "manual"
	RestoreTypeAutomated RestoreType = "automated"
)

const OperatorDomain = "backup.datarestor-operator.com"

// Common status fields for all CRDs
type CommonStatus struct {
	// Current phase of the operation
	// +optional
	Phase Phase `json:"phase,omitempty"`

	// Error message if operation failed
	// +optional
	Error string `json:"error,omitempty"`

	// Time when the operation started
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// Time when the operation completed
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Time when the backup failed
	// +optional
	FailedTime *metav1.Time `json:"failedTime,omitempty"`

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}
