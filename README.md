# PVC Backup Operator

Kubernetes operator for automated PVC backup/restore with Restic. Simple, secure, and efficient.

## What it does

- **Automated backups** on schedule (cron-based)
- **Multiple targets** (S3, NFS, SFTP, Azure, GCS, etc.)
- **Global deduplication** and compression
- **End-to-end encryption**
- **Auto-restore** for new PVCs
- **Smart retention** policies

## Quick Start

```bash
# Install
make install && make deploy

# Or run locally
make run
```

## Example Config

```yaml
apiVersion: storage.cheap-man-ha-store.com/v1alpha1
kind: PVCBackup
metadata:
  name: db-backup
spec:
  pvcSelector:
    labelSelector:
      matchLabels:
        app: database
        backup: "true"
    namespaces: [default, production]

  backupTargets:
    - name: s3-primary
      priority: 1
      restic:
        repository: "s3:s3.amazonaws.com/my-bucket"
        password: "your-password"
        env:
          - name: AWS_ACCESS_KEY_ID
            value: "your-key"
          - name: AWS_SECRET_ACCESS_KEY
            value: "your-secret"
        tags: [production, daily]

    - name: nfs-local
      priority: 2
      restic:
        repository: "local:/mnt/backup"
        password: "your-password"
        flags: ["--no-lock"]

  schedule:
    cron: "0 2 * * *" # Daily at 2 AM
    waitForHealthy: true

  autoRestore: true
  retention:
    maxSnapshots: 30
    maxAge: "90d"
```

## Repository Types

### S3/S3-Compatible

```yaml
backupTargets:
  - name: aws-s3
    priority: 1
    restic:
      repository: "s3:s3.amazonaws.com/bucket-name"
      password: "your-password"
      env:
        - name: AWS_ACCESS_KEY_ID
          value: "your-key"
        - name: AWS_SECRET_ACCESS_KEY
          value: "your-secret"
        - name: AWS_DEFAULT_REGION
          value: "us-west-2"
      tags: [aws, production]

  - name: minio
    priority: 2
    restic:
      repository: "s3:minio.example.com:9000/bucket-name"
      password: "your-password"
      env:
        - name: AWS_ACCESS_KEY_ID
          value: "minio-key"
        - name: AWS_SECRET_ACCESS_KEY
          value: "minio-secret"
        - name: AWS_ENDPOINT
          value: "http://minio.example.com:9000"
      tags: [minio, local]
```

### NFS/Local Storage

```yaml
backupTargets:
  - name: nfs-backup
    priority: 1
    restic:
      repository: "local:/mnt/nfs/backup/restic-repo"
      password: "your-password"
      flags: ["--no-lock"] # Important for NFS
      tags: [nfs, local]

  - name: local-fast
    priority: 2
    restic:
      repository: "local:/mnt/fast-ssd/backup"
      password: "your-password"
      tags: [local, fast-access]
```

### NFS Server Examples

```yaml
backupTargets:
  - name: nfs-primary
    priority: 1
    restic:
      repository: "local:/mnt/nfs-primary/backup"
      password: "your-password"
      flags: ["--no-lock"]
      tags: [nfs, primary]
    # Mount this NFS in your pod:
    # - name: nfs-backup
    #   mountPath: /mnt/nfs-primary
    #   nfs:
    #     server: nfs.example.com
    #     path: /backup

  - name: nfs-secondary
    priority: 2
    restic:
      repository: "local:/mnt/nfs-secondary/backup"
      password: "your-password"
      flags: ["--no-lock"]
      tags: [nfs, secondary]
    # Mount this NFS in your pod:
    # - name: nfs-backup-2
    #   mountPath: /mnt/nfs-secondary
    #   nfs:
    #     server: nfs2.example.com
    #     path: /backup
```

**Note**: NFS repositories use `local:/mnt/path` because Restic sees the mounted NFS as a local directory. The actual NFS server and path are configured in the Kubernetes volume mount.

### Dynamic NFS Mounting

For better flexibility, the operator can dynamically mount NFS shares when needed:

```yaml
backupTargets:
  - name: nfs-dynamic
    priority: 1
    restic:
      repository: "local:/mnt/backup"
      password: "your-password"
      flags: ["--no-lock"]
      tags: [nfs, dynamic]
    nfs:
      server: "nfs.example.com"
      path: "/backup"
      mountOptions: ["nfsvers=4", "soft"]
```

**Note**: The operator will automatically mount/unmount NFS shares as needed, so you don't need to pre-mount all possible NFS locations in your deployment.

### How Dynamic NFS Mounting Works

1. **No Pre-mounting**: The operator doesn't need NFS shares mounted at startup
2. **On-Demand Mounting**: When a backup starts, the operator mounts the NFS share to `/mnt/backup`
3. **Restic Access**: Restic sees the mounted NFS as a local directory at `local:/mnt/backup`
4. **Auto-Unmounting**: After backup completes, the NFS share is unmounted
5. **Multiple Targets**: Each NFS target can have different servers, paths, and mount options

This approach gives you:

- **Flexibility**: Add/remove NFS servers without operator restarts
- **Efficiency**: Only mount NFS when actually backing up
- **Scalability**: Support many NFS targets without deployment complexity
- **Security**: NFS credentials can be managed per target

### Implementation Details

The operator now includes:

- **NFSMounter**: Creates temporary pods to mount NFS shares using Kubernetes volumes
- **Automatic Cleanup**: Ensures NFS mount pods are deleted after operations
- **Kubernetes Native**: Uses standard Kubernetes patterns instead of elevated capabilities
- **Error Handling**: Graceful fallback if NFS mounting fails
- **Repository Path Resolution**: Automatically adjusts Restic repository paths for mounted NFS

**Note**: The operator creates temporary privileged pods for NFS mounting, eliminating the need for elevated capabilities in the main operator container.

## Advanced Options

### Custom Flags & Performance

```yaml
backupTargets:
  - name: optimized-backup
    priority: 1
    restic:
      repository: "s3:s3.amazonaws.com/my-bucket"
      password: "your-password"
      env:
        - name: AWS_ACCESS_KEY_ID
          value: "your-key"
        - name: AWS_SECRET_ACCESS_KEY
          value: "your-secret"
      flags:
        - "--compression=auto"
        - "--parallel=4"
        - "--exclude=*.tmp"
        - "--exclude=*.log"
        - "--exclude=node_modules"
      tags: [production, optimized]
      host: "k8s-cluster-1"
```

### Multi-Cloud Strategy

```yaml
backupTargets:
  - name: aws-primary
    priority: 1
    restic:
      repository: "s3:s3.amazonaws.com/primary"
      password: "primary-password"
      env:
        - name: AWS_ACCESS_KEY_ID
          value: "primary-key"
        - name: AWS_SECRET_ACCESS_KEY
          value: "primary-secret"
      tags: [primary, production]

  - name: local-archive
    priority: 3
    restic:
      repository: "local:/mnt/archive/backup"
      password: "archive-password"
      tags: [local, long-term, archive]
```

## Prerequisites

- Kubernetes 1.24+
- VolumeSnapshot CRD
- Restic binary in container
- NFS client tools available in cluster (for temporary mount pods)

## Development

```bash
make manifests  # Generate CRDs
make build      # Build binary
make test       # Run tests
make run        # Run locally
```

## Project Structure

```
├── api/v1alpha1/        # API types
├── internal/controller/  # Main logic
├── config/              # K8s manifests
└── cmd/                 # Entry point
```

## Status & Monitoring

```bash
# Check status
kubectl get pvcbackup db-backup -o yaml

# View logs
kubectl logs -n cheap-man-ha-store-system deployment/cheap-man-ha-store-controller-manager
```

## Need Help?

- Check [Restic docs](https://restic.readthedocs.io/)
- Look at sample configs in `config/samples/`
- Open an issue on GitHub
