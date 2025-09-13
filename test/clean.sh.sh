#!/bin/bash

NAMESPACE="backup-test-config"
API_GROUP="backup.datarestor-operator.com"

# List of custom resource kinds to check
KINDS=(
  "backupconfigs"
  "resticrepositories"
  "resticbackups"
  "resticrestores"
  "tasks"
)

for kind in "${KINDS[@]}"; do
  resource_type="${kind}.${API_GROUP}"
  echo "Processing ${resource_type}..."

  # Get all resources for the kind in the namespace
  resources=$(kubectl get "${resource_type}" -n "${NAMESPACE}" -o jsonpath='{.items[*].metadata.name}')

  if [ -z "$resources" ]; then
    echo "No resources found for ${resource_type} in namespace ${NAMESPACE}."
    continue
  fi

  for resource in $resources; do
    echo "Removing finalizers from ${resource_type}/${resource} in namespace ${NAMESPACE}"
    kubectl patch "${resource_type}" "${resource}" -n "${NAMESPACE}" --type='merge' -p '{"metadata":{"finalizers":[]}}'
  done
done

echo "Finalizer removal process completed."
echo "The namespace ${NAMESPACE} should now be deleted."