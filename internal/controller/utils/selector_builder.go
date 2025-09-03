package utils

import (
	v1 "github.com/sladg/datarestor-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Common selector patterns for finding resources

// SelectorInNamespace creates a simple namespace selector
func SelectorInNamespace(namespace string) v1.Selector {
	return v1.Selector{
		Namespaces: []string{namespace},
	}
}

// SelectorForResource creates a selector for a specific resource in a namespace
func SelectorForResource(namespace string, name *string, matchLabels map[string]string) v1.Selector {
	selector := v1.Selector{
		Namespaces: []string{namespace},
	}

	if matchLabels != nil {
		selector.LabelSelector = &metav1.LabelSelector{
			MatchLabels: matchLabels,
		}
	} else if name != nil {
		selector.Names = []string{*name}
	}

	return selector
}

// Jobs will use OwnerReferences which is the Kubernetes standard way
// of tracking ownership, so we don't need special selectors for them
