package restic

import (
	"reflect"
	"testing"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeRestoreArgs(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "test-ns",
		},
	}
	repos := []v1.RepositorySpec{
		{Priority: 1, Target: "repo1"},
		{Priority: 0, Target: "repo0"},
		{Priority: 2, Target: "s3:http://localhost:9000/onsite-backups"},
	}

	testCases := []struct {
		name       string
		params     MakeArgsParams
		expected   []string
		setupMocks func()
	}{
		{
			name: "Annotation with repo priority, host, and snapshot name",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "0|my-host|my-snapshot",
			},
			expected: []string{"restore", "--repo=repo0", "--host=my-host", "--tag", "name=my-snapshot", "latest", "--target", "."},
		},
		{
			name: "Annotation with repo target, host, and snapshot name",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "repo1|my-host|my-snapshot",
			},
			expected: []string{"restore", "--repo=repo1", "--host=my-host", "--tag", "name=my-snapshot", "latest", "--target", "."},
		},
		{
			name: "Annotation with host and snapshot name",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "my-host|my-snapshot",
			},
			expected: []string{"restore", "--repo=repo0", "--host=my-host", "--tag", "name=my-snapshot", "latest", "--target", "."},
		},
		{
			name: "Annotation with snapshot name only",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "my-snapshot",
			},
			expected: []string{"restore", "--repo=repo0", "--host=test-ns-test-pvc", "my-snapshot", "--target", "."},
		},
		{
			name: "Annotation with 'latest'",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "latest",
			},
			expected: []string{"restore", "--repo=repo0", "--host=test-ns-test-pvc", "latest", "--target", "."},
		},
		{
			name: "Annotation with repo and 'latest'",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "1|my-host|latest",
			},
			expected: []string{"restore", "--repo=repo1", "--host=my-host", "latest", "--target", "."},
		},
		{
			name: "Empty annotation",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "",
			},
			expected: []string{"restore", "--repo=repo0", "--host=test-ns-test-pvc", "latest", "--target", "."},
		},
		{
			name: "Annotation with all empty parts",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "||",
			},
			expected: []string{"restore", "--repo=repo0", "--host=test-ns-test-pvc", "latest", "--target", "."},
		},
		{
			name: "Annotation with repo and empty host and snapshot",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "repo1||",
			},
			expected: []string{"restore", "--repo=repo1", "--host=test-ns-test-pvc", "latest", "--target", "."},
		},
		{
			name: "Annotation with repo, host, and empty snapshot",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "repo1|some-other-host|",
			},
			expected: []string{"restore", "--repo=repo1", "--host=some-other-host", "latest", "--target", "."},
		},
		{
			name: "Annotation with single pipe",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "|",
			},
			expected: []string{"restore", "--repo=repo0", "--host=test-ns-test-pvc", "latest", "--target", "."},
		},
		{
			name: "Annotation with empty repo and snapshot",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "|my-host|",
			},
			expected: []string{"restore", "--repo=repo0", "--host=my-host", "latest", "--target", "."},
		},
		{
			name: "Annotation with empty repo and host",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "||my-snapshot",
			},
			expected: []string{"restore", "--repo=repo0", "--host=test-ns-test-pvc", "--tag", "name=my-snapshot", "latest", "--target", "."},
		},
		{
			name: "User-reported annotation format",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "s3:http://localhost:9000/onsite-backups|default-simple-app-data|now",
			},
			expected: []string{"restore", "--repo=s3:http://localhost:9000/onsite-backups", "--host=default-simple-app-data", "latest", "--target", "."},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setupMocks != nil {
				tc.setupMocks()
			}
			result := MakeRestoreArgs(tc.params)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected args %v, but got %v", tc.expected, result)
			}
		})
	}
}

func TestMakeBackupArgs(t *testing.T) {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pvc",
			Namespace: "test-ns",
		},
	}
	repos := []v1.RepositorySpec{
		{Priority: 1, Target: "repo1"},
		{Priority: 0, Target: "repo0"},
	}

	testCases := []struct {
		name     string
		params   MakeArgsParams
		expected []string
	}{
		{
			name: "Annotation with a backup name",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "my-backup",
				TaskName:     "task-123",
			},
			expected: []string{"backup", "--repo=repo0", "--host=test-ns-test-pvc", "--tag", "name=my-backup", constants.MountPath},
		},
		{
			name: "Annotation 'true'",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "true",
				TaskName:     "task-123",
			},
			expected: []string{"backup", "--repo=repo0", "--host=test-ns-test-pvc", "--tag", "name=task-123", constants.MountPath},
		},
		{
			name: "Annotation 'now'",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "now",
				TaskName:     "task-456",
			},
			expected: []string{"backup", "--repo=repo0", "--host=test-ns-test-pvc", "--tag", "name=task-456", constants.MountPath},
		},
		{
			name: "Empty annotation",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "",
				TaskName:     "task-789",
			},
			expected: []string{"backup", "--repo=repo0", "--host=test-ns-test-pvc", "--tag", "name=task-789", constants.MountPath},
		},
		{
			name: "Annotation with repo priority, host and backup name",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "1|my-host|my-backup",
				TaskName:     "task-abc",
			},
			expected: []string{"backup", "--repo=repo1", "--host=my-host", "--tag", "name=my-backup", constants.MountPath},
		},
		{
			name: "Annotation with repo target, host and backup name",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "repo1|my-host|my-backup",
				TaskName:     "task-def",
			},
			expected: []string{"backup", "--repo=repo1", "--host=my-host", "--tag", "name=my-backup", constants.MountPath},
		},
		{
			name: "Annotation with host and backup name",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "my-host|my-backup",
				TaskName:     "task-jkl",
			},
			expected: []string{"backup", "--repo=repo0", "--host=my-host", "--tag", "name=my-backup", constants.MountPath},
		},
		{
			name: "Annotation with repo, host and 'now' as name",
			params: MakeArgsParams{
				Repositories: repos,
				TargetPVC:    pvc,
				Annotation:   "repo1|my-host|now",
				TaskName:     "task-xyz",
			},
			expected: []string{"backup", "--repo=repo1", "--host=my-host", "--tag", "name=task-xyz", constants.MountPath},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := MakeBackupArgs(tc.params)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("Expected args %v, but got %v", tc.expected, result)
			}
		})
	}
}
