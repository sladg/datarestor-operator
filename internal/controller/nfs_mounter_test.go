package controller

import (
	"testing"

	storagev1alpha1 "github.com/cheap-man-ha-store/cheap-man-ha-store/api/v1alpha1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestNFSMounter_NewNFSMounter(t *testing.T) {
	// Create a fake client for testing
	s := scheme.Scheme
	client := fake.NewClientBuilder().WithScheme(s).Build()

	mounter := NewNFSMounter(client)
	if mounter == nil {
		t.Fatal("NewNFSMounter returned nil")
	}
	if mounter.client == nil {
		t.Fatal("client not initialized")
	}
}

func TestNFSMounter_NeedsNFSMounting(t *testing.T) {
	// Create a fake client for testing
	s := scheme.Scheme
	client := fake.NewClientBuilder().WithScheme(s).Build()

	resticClient := NewResticClient(client)

	tests := []struct {
		name     string
		target   storagev1alpha1.BackupTarget
		expected bool
	}{
		{
			name: "no NFS config",
			target: storagev1alpha1.BackupTarget{
				Name: "test",
				Restic: &storagev1alpha1.ResticConfig{
					Repository: "s3:s3.amazonaws.com/bucket",
				},
			},
			expected: false,
		},
		{
			name: "with NFS config",
			target: storagev1alpha1.BackupTarget{
				Name: "test",
				Restic: &storagev1alpha1.ResticConfig{
					Repository: "local:/mnt/backup",
				},
				NFS: &storagev1alpha1.NFSConfig{
					Server: "nfs.example.com",
					Path:   "/backup",
				},
			},
			expected: true,
		},
		{
			name: "NFS config without server",
			target: storagev1alpha1.BackupTarget{
				Name: "test",
				Restic: &storagev1alpha1.ResticConfig{
					Repository: "local:/mnt/backup",
				},
				NFS: &storagev1alpha1.NFSConfig{
					Path: "/backup",
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resticClient.needsNFSMounting(tt.target)
			if result != tt.expected {
				t.Errorf("needsNFSMounting() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNFSMounter_GetMountPath(t *testing.T) {
	// Create a fake client for testing
	s := scheme.Scheme
	client := fake.NewClientBuilder().WithScheme(s).Build()

	mounter := NewNFSMounter(client)

	// Test that GetMountPath returns a predictable path
	mountPath, exists := mounter.GetMountPath("test")
	if !exists {
		t.Error("Expected mount path to exist")
	}
	if mountPath != "/mnt/nfs-test" {
		t.Errorf("Expected mount path '/mnt/nfs-test', got %s", mountPath)
	}
}
