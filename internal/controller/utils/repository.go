package utils

import v1 "github.com/sladg/datarestor-operator/api/v1alpha1"

func FindRepositorySpec(repos []v1.RepositorySpec, repoStatus *v1.RepositoryStatus) *v1.RepositorySpec {
	for _, repo := range repos {
		if repo.Target == repoStatus.Target {
			return &repo
		}
	}
	return nil
}
