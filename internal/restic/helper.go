package restic

import (
	"fmt"
	"strings"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
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

func convertStringToNameArgs(params MakeArgsParams) []string {
	if params.Annotation == "true" || params.Annotation == "now" || params.Annotation == "" {
		return []string{"latest"}
	}

	// @TODO: Implement
	// Check name across repositories, if found unique, use it. Otherwise prefer most priority / now repo

	if contains := utils.Contains(params.Repositories[0].Status.Backups, params.Annotation); contains {
		return []string{fmt.Sprintf("--tag name=%s", params.Annotation), "latest"}
	}

	// Assume it's restic's snapshot ID
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
	return arguments
}

func MakeBackupArgs(params MakeArgsParams) []string {
	// @TODO: Add check for duplicated name
	arguments := []string{"backup"}
	arguments = append(arguments, convertStringToRepoNameArgs(params)...)
	arguments = append(arguments, convertStringToHostArgs(params)...)
	arguments = append(arguments, convertParamsToBackupName(params)...)
	return arguments
}
