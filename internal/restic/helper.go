package restic

import (
	"fmt"
	"strings"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type MakeArgsParams struct {
	Repositories []v1.RepositorySpec
	Env          []corev1.EnvVar
	TargetPVC    *corev1.PersistentVolumeClaim
	Annotation   string
}

func isNumber(s string) (int32, bool) {
	var num int32
	_, err := fmt.Sscanf(s, "%d", &num)
	return num, err == nil
}

func GetHostFromPVC(pvc *corev1.PersistentVolumeClaim) string {
	return fmt.Sprintf("%s-%s", pvc.Namespace, pvc.Name)
}

func convertStringToRepoNameArgs(params MakeArgsParams) []string {
	parts := strings.Split(params.Annotation, "#")

	if len(parts) == 3 && parts[0] != "" {
		if num, isNumber := isNumber(parts[0]); isNumber {
			for _, repo := range params.Repositories {
				if repo.Priority == num {
					return []string{fmt.Sprintf("--repo=%s", repo.Target)}
				}
			}
		}

		return []string{fmt.Sprintf("--repo=%s", parts[0])}
	}

	priorityRepo := params.Repositories[0]
	for _, repo := range params.Repositories {
		if repo.Priority < priorityRepo.Priority {
			priorityRepo = repo
		}
	}

	return []string{fmt.Sprintf("--repo=%s", priorityRepo.Target)}
}

// SelectRepository returns the repository selected by the same rules used by
// convertStringToRepoNameArgs. When a numeric priority is provided, it returns
// the repository with that priority. When a literal repo target is provided,
// it will match by Target; if not found, it falls back to the priority repo.
func SelectRepository(params MakeArgsParams) v1.RepositorySpec {
	parts := strings.Split(params.Annotation, "#")

	// Find lowest priority repo as default
	priorityRepo := params.Repositories[0]
	for _, repo := range params.Repositories {
		if repo.Priority < priorityRepo.Priority {
			priorityRepo = repo
		}
	}

	if len(parts) == 3 && parts[0] != "" {
		if num, isNum := isNumber(parts[0]); isNum {
			for _, repo := range params.Repositories {
				if repo.Priority == num {
					return repo
				}
			}
			return priorityRepo
		}
		// Try to match by explicit target
		for _, repo := range params.Repositories {
			if repo.Target == parts[0] {
				return repo
			}
		}
		return priorityRepo
	}

	return priorityRepo
}

func convertStringToHostArgs(params MakeArgsParams) []string {
	parts := strings.Split(params.Annotation, "#")
	if len(parts) == 3 && parts[1] != "" {
		return []string{fmt.Sprintf("--host=%s", parts[1])}
	}

	if len(parts) == 2 && parts[0] != "" {
		return []string{fmt.Sprintf("--host=%s", parts[0])}
	}

	return []string{fmt.Sprintf("--host=%s", GetHostFromPVC(params.TargetPVC))}
}

var LatestAcceptableValues = []string{"latest", "true", "now", ""}

func convertStringToNameArgs(params MakeArgsParams) []string {
	parts := strings.Split(params.Annotation, "#")
	if len(parts) == 3 {
		name := parts[2]
		if utils.Contains(LatestAcceptableValues, name) {
			return []string{"latest"}
		}
		return []string{"--tag", fmt.Sprintf("name=%s", name), "latest"}
	}

	if len(parts) == 2 {
		name := parts[1]
		if utils.Contains(LatestAcceptableValues, name) {
			return []string{"latest"}
		}
		return []string{"--tag", fmt.Sprintf("name=%s", name), "latest"}
	}

	if utils.Contains(LatestAcceptableValues, params.Annotation) {
		return []string{"latest"}
	}

	// Treat as snapshot ID/prefix
	return []string{params.Annotation}
}

func convertParamsToBackupName(params MakeArgsParams) []string {
	if params.Annotation == "true" || params.Annotation == "now" || params.Annotation == "" {
		// If nothing is provided, generate a short random name
		return []string{"--tag", fmt.Sprintf("name=%s", uuid.NewUUID()[:6])}
	}

	return []string{"--tag", fmt.Sprintf("name=%s", params.Annotation)}
}

func MakeRestoreArgs(params MakeArgsParams) []string {
	// @TODO: Add checks for if exists. We should pass the restic's ID into jobs for simplicity
	arguments := []string{"restore"}
	arguments = append(arguments, convertStringToRepoNameArgs(params)...)
	arguments = append(arguments, convertStringToHostArgs(params)...)
	arguments = append(arguments, convertStringToNameArgs(params)...)
	arguments = append(arguments, "--target", ".") // For restore, restic expects --target <path>. We restore into current directory as we rely on the PVC's mount path.
	return arguments
}

func MakeBackupArgs(params MakeArgsParams) []string {
	// @TODO: Add check for duplicated name
	arguments := []string{"backup"}
	arguments = append(arguments, convertStringToRepoNameArgs(params)...)
	arguments = append(arguments, convertStringToHostArgs(params)...)
	arguments = append(arguments, convertParamsToBackupName(params)...)
	arguments = append(arguments, constants.MountPath) // For backup, restic expects the source path as a positional arg.
	return arguments
}
