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

// ========================================
// PHASE CONSTANTS (Grouped by CRD)
// ========================================

// Generic operation phases (used across all CRDs)
type Phase string

const (
	PhaseUnknown   = ""
	PhasePending   = "Pending"   // Initial state, not yet started
	PhaseRunning   = "Running"   // Operation in progress
	PhaseCompleted = "Completed" // Operation finished successfully
	PhaseFailed    = "Failed"    // Operation failed
)

// ========================================
// BACKUP TYPE DEFINITIONS
// ========================================

// BackupType defines the type of backup operation
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

// ========================================
// OPERATIONAL CONSTANTS
// ========================================

// Note: OperatorDomain is now defined in internal/constants/constants.go to avoid duplication

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

	// Conditions represent the latest available observations
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// JobReference holds reference to a Kubernetes Job
type JobReference struct {
	// Reference to the Kubernetes Job
	// +optional
	JobRef *corev1.LocalObjectReference `json:"jobRef,omitempty"`
}

type PersistentVolumeClaimRef struct {
	// +required
	Name string `json:"name"`

	// +required
	Namespace string `json:"namespace"`
}

// ResticRepositoryRef contains a reference to a ResticRepository
type ResticRepositoryRef struct {
	// Name of the ResticRepository
	// +required
	Name string `json:"name"`
}
