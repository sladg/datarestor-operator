# Example Configurations

This document focuses on the recommended usage pattern: define `BackupConfig` and trigger ad-hoc operations via annotations. Creating `ResticRepository`, `ResticBackup`, or `ResticRestore` directly is possible but not recommended for normal operations.

## BackupConfig Example

Below is a complete example of a `BackupConfig` resource that demonstrates common usage patterns.

```yaml
apiVersion: backup.datarestor-operator.com/v1alpha1
kind: BackupConfig
metadata:
  name: homelab-backups
  namespace: database
spec:
  # A list of selectors to find PVCs. A PVC will be backed up if it matches ANY of these selectors.
  selectors:
    # First selector: Match all PVCs in the 'database' namespace with the 'app: postgres' label.
    - namespaces:
        - "database"
      labelSelector:
        matchLabels:
          app: postgres
    # Second selector: Match any PVC in the 'frontend' namespace, regardless of labels.
    - namespaces:
        - "frontend"

  # Enable auto-restore for new PVCs that match one of the selectors above.
  # The first BackupConfig controller to "claim" a new PVC will be the one
  # to perform the auto-restore.
  autoRestore: true

  # Define one or more backup destinations, each with its own schedule.
  # Auto-restore uses the highest-priority backupTarget (lowest priority number) by default.
  backupTargets:
    # Primary, off-site S3 backup running daily with a long-term retention policy.
    - name: offsite-s3-long-term
      priority: 10
      schedule:
        cron: "0 2 * * *" # Daily at 2 AM
      restic:
        # The S3 bucket URL for the restic repository.
        repository: s3:s3.amazonaws.com/my-backup-bucket/postgres
        # Reference to a Kubernetes Secret in the same namespace that holds the
        # repository password. The secret must have a key named 'password'.
        passwordSecretRef:
          name: postgres-repo-password
          key: password
        # It's recommended to use a secret for credentials in a real environment.
        # This example uses environment variables for simplicity.
        env:
          - name: AWS_ACCESS_KEY_ID
            value: "your-access-key"
          - name: AWS_SECRET_ACCESS_KEY
            value: "your-secret-key"
        # [Advanced] Pass additional flags directly to Restic.
        # This is powerful but can break backups if used incorrectly.
        # Example: Exclude a specific directory from this target's backups.
        additionalFlags:
          - "--exclude=/var/log"
      # Keep daily, weekly, and monthly backups for an extended period.
      retention:
        keepDaily: 14
        keepWeekly: 8
        keepMonthly: 12
        prune: true

    # Secondary, on-site MinIO backup running hourly for quick restores with a shorter retention.
    - name: onsite-minio-short-term
      priority: 20
      schedule:
        cron: "0 * * * *" # Every hour
      restic:
        repository: s3:http://minio.local:9000/postgres-backups
        passwordSecretRef:
          name: postgres-repo-password
          key: password
        additionalFlags:
          - "--read-concurrency=10"
        env:
          - name: AWS_ACCESS_KEY_ID
            value: "minio-access-key"
          - name: AWS_SECRET_ACCESS_KEY
            value: "minio-secret-key"
      # Keep only the last 7 daily backups.
      retention:
        keepDaily: 7
        prune: true
```

<!-- Advanced standalone CR examples intentionally omitted per best practices. -->

## Manual Annotation Examples

### Trigger a one-time backup on a PVC

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-data
  namespace: database
  annotations:
    backup.autorestore.com/manual-backup: "now"
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 20Gi
```

### Trigger backups for all PVCs managed by a BackupConfig

```yaml
apiVersion: backup.datarestor-operator.com/v1alpha1
kind: BackupConfig
metadata:
  name: homelab-backups
  namespace: database
  annotations:
    backup.autorestore.com/manual-backup: "manual-all-$(date +%Y%m%d-%H%M%S)"
spec:
  selectors:
    - namespaces: ["database"]
  autoRestore: true
  backupTargets:
    - name: offsite-s3-long-term
      priority: 10
      schedule:
        cron: "0 2 * * *"
      restic:
        repository: s3:s3.amazonaws.com/my-backup-bucket/postgres
        passwordSecretRef:
          name: postgres-repo-password
          key: password
```

### Trigger a restore into a PVC via annotation

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-restore
  namespace: database
  annotations:
    backup.autorestore.com/restore-from-backup: "db-backup-manual-001"
spec:
  accessModes: ["ReadWriteOnce"]
  resources:
    requests:
      storage: 20Gi
```
