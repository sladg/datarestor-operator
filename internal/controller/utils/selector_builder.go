package utils

import (
	backupv1alpha1 "github.com/sladg/autorestore-backup-operator/api/v1alpha1"
)

// Common selector patterns for finding resources

// SelectorInNamespace creates a simple namespace selector
func SelectorInNamespace(namespace string) backupv1alpha1.Selector {
	return backupv1alpha1.Selector{
		Namespaces: []string{namespace},
	}
}

// SelectorForResource creates a selector for a specific resource in a namespace
func SelectorForResource(name, namespace string) backupv1alpha1.Selector {
	return backupv1alpha1.Selector{
		Names:      []string{name},
		Namespaces: []string{namespace},
	}
}

// Jobs will use OwnerReferences which is the Kubernetes standard way
// of tracking ownership, so we don't need special selectors for them
