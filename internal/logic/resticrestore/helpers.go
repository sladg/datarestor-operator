package resticrestore

import (
	"context"
	"fmt"
	"strings"

	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	"github.com/sladg/datarestor-operator/internal/constants"
	"github.com/sladg/datarestor-operator/internal/controller/utils"
	corev1 "k8s.io/api/core/v1"
)

func ConvertStringToSnapshotID(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore, value string) []string {
	if value == "" {
		return []string{"latest"}
	}

	if value == "true" || value == "now" || value == "" {
		return []string{"latest"}
	}

	backup, err := utils.GetResource[*v1.ResticBackup](ctx, deps.Client, restore.Namespace, value)
	if err == nil && backup != nil {
		// If backup is found, use the tag as filter for restic restore and use latest as we expect to find single snapshot
		return []string{fmt.Sprintf("--name=%s", value), "latest"}
	}

	return []string{value}
}

/*
If annotationValue is "true" or "now" or empty - use latest snapshot
If annotationValue is found in ResticBackup - use the tag as filter for restic restore
If annotationValue is not found in ResticBackup - assume it is a snapshot ID
If annotationValue is namespace-pvcName#snapshotID - convert correctly
*/
func GetRestoreArgs(ctx context.Context, deps *utils.Dependencies, restore *v1.ResticRestore) []string {
	pvcRef := restore.Spec.TargetPVC
	annotationValue := restore.GetAnnotations()[constants.AnnotationManualRestore]

	pvc, _ := utils.GetResource[*corev1.PersistentVolumeClaim](ctx, deps.Client, pvcRef.Namespace, pvcRef.Name)
	pvcAnnotationValue := pvc.GetAnnotations()[constants.AnnotationManualRestore]

	if pvcAnnotationValue != "" {
		// We prefer the annotation on the PVC over the annotation on the restore
		annotationValue = pvcAnnotationValue
	}

	if strings.Contains(annotationValue, "#") {
		host := strings.Split(annotationValue, "#")[0]
		snapshotID := strings.Split(annotationValue, "#")[1]

		args := []string{fmt.Sprintf("--host=%s", host), fmt.Sprintf("--tag=%s", host)}
		args = append(args, ConvertStringToSnapshotID(ctx, deps, restore, snapshotID)...)
		return args
	}

	return ConvertStringToSnapshotID(ctx, deps, restore, annotationValue)
}
