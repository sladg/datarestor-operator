# PVC Backup Operator

A Kubernetes operator for automated PVC backup and restore operations with support for multiple backup targets and priorities.

## Features

- **Automated PVC Backup**: Schedule-based backups using cron expressions
- **Multiple Backup Targets**: Support for S3 and NFS with priority-based selection
- **Data Integrity**: Option to stop pods during backup for consistent snapshots
- **Automatic Restore**: Restore new PVCs from existing backups
- **Init Container Support**: Wait for restore completion before starting applications
- **Retention Policies**: Configurable snapshot retention per target
- **Health Checks**: Wait for pod health before backup operations
- **Restic Integration**: Plain data backup using restic for manual intervention

## Architecture

The operator watches for:

- `PVCBackup` custom resources
- `PersistentVolumeClaim` objects
- `Pod` objects using PVCs

## Installation

### Prerequisites

- Kubernetes 1.24+
- VolumeSnapshot CRD (for volume snapshots)
- S3-compatible storage or NFS server

### Deploy the Operator

```bash
# Install CRDs
make install

# Deploy the operator
make deploy

# Or build and run locally
make run
```

## Usage

### Basic PVCBackup Configuration

```yaml
apiVersion: storage.cheap-man-ha-store.com/v1alpha1
kind: PVCBackup
metadata:
  name: database-backup
  namespace: default
spec:
  # Select PVCs to backup
  pvcSelector:
    labelSelector:
      matchLabels:
        backup.enabled: "true"
        app: database
    namespaces:
      - default
      - database

  # Backup targets with priorities
  backupTargets:
    - name: nfs-primary
      priority: 1
      type: nfs
      nfs:
        server: "192.168.1.100"
        path: "/backups"
      retention:
        maxSnapshots: 10
        maxAge: "168h"

    - name: s3-secondary
      priority: 2
      type: s3
      s3:
        bucket: "pvc-backups"
        region: "us-west-2"
        accessKeyID: "your-access-key"
        secretAccessKey: "your-secret-key"
      retention:
        maxSnapshots: 30

  # Backup schedule
  schedule:
    cron: "0 2 * * *" # Daily at 2 AM
    stopPods: true # Stop pods for data integrity
    waitForHealthy: true

  # Enable automatic restore
  autoRestore: true

  # Init container for restore waiting
  initContainer:
    image: "busybox:1.35"
    command: ["/bin/sh"]
    args:
      - "-c"
      - "echo 'Waiting for PVC restore...' && sleep 30"
```

### PVC Selection

The operator supports multiple ways to select PVCs:

1. **Label Selector**: Use Kubernetes label selectors
2. **Namespace Filtering**: Specify namespaces to monitor
3. **Name Filtering**: List specific PVC names

### Backup Targets

#### NFS Target

```yaml
backupTargets:
  - name: nfs-backup
    priority: 1
    type: nfs
    nfs:
      server: "192.168.1.100"
      path: "/backups"
      mountOptions:
        - "nfsvers=4"
        - "soft"
```

#### S3 Target

```yaml
backupTargets:
  - name: s3-backup
    priority: 2
    type: s3
    s3:
      bucket: "pvc-backups"
      region: "us-west-2"
      endpoint: "https://s3.us-west-2.amazonaws.com"
      accessKeyID: "your-key"
      secretAccessKey: "your-secret"
      pathPrefix: "backups"
```

### Retention Policies

Configure how many snapshots to keep:

```yaml
retention:
  maxSnapshots: 10 # Keep max 10 snapshots
  maxAge: "168h" # Keep snapshots for 7 days
```

### Automatic Restore

Enable automatic restore for new PVCs:

```yaml
spec:
  autoRestore: true

  # PVCs with this label will be automatically restored
  pvcSelector:
    labelSelector:
      matchLabels:
        backup.restore: "true"
```

## Development

### Building

```bash
# Generate manifests
make manifests

# Build the operator
make build

# Run tests
make test

# Run locally
make run
```

### Project Structure

```
├── api/v1alpha1/           # API types and CRD definitions
├── internal/controller/     # Controller implementation
├── config/                 # Kubernetes manifests
│   ├── crd/               # Custom Resource Definitions
│   ├── rbac/              # RBAC configuration
│   └── samples/           # Example configurations
└── cmd/                   # Main application entry point
```

## Monitoring

The operator provides status information:

```bash
kubectl get pvcbackup database-backup -o yaml
```

Status fields include:

- `managedPVCs`: List of PVCs being managed
- `lastBackup`: Timestamp of last backup
- `lastRestore`: Timestamp of last restore
- `backupStatus`: Current backup status
- `restoreStatus`: Current restore status
- `successfulBackups`: Count of successful backups
- `failedBackups`: Count of failed backups

## Troubleshooting

### Check Operator Logs

```bash
kubectl logs -n cheap-man-ha-store-system deployment/cheap-man-ha-store-controller-manager
```

### Verify CRD Installation

```bash
kubectl get crd pvcbackups.storage.cheap-man-ha-store.com
```

### Check RBAC Permissions

```bash
kubectl auth can-i create pvcbackups --as=system:serviceaccount:cheap-man-ha-store-system:cheap-man-ha-store-controller-manager
```
