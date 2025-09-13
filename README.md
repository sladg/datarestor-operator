# DataRestor Backup Operator

## 🚧 UNDER CONSTRUCTION 🚧

---

An operator for automated PVC backup and restore with Restic.
A cheap, self-host-friendly operator that copies your volumes data on a schedule and populates them back automagically whenever you rebuild the cluster and your volumes are gone.

### Why?

Because I hate thinking of Disaster Recovery. When shit hits fan, I want to just re-apply everything without googling around and updating yamls one by one while everything is on fire. I want single YAML to handle restoring everything without me needing to dig around documentations and trying to remember what needs to be manually updated where.

- Configure once with a `Config`, then forget about it.
- New PVC appears? It’s backed up automatically.
- Cluster goes boom? Re-deploy the operator and it re-hydrates your volumes — no wall of YAML, no `kubectl cp`, no manual intervention.

Perfect for disaster recovery, painless migrations, simple rollbacks on failed upgrades.

## Philosophy

- Existing YAMLs work in DR scenarios.
- Annotations are way to manually interve when necessary.
- Single YAML for configuration of this operator.

## Features

- **Automated Backups**: Schedule backups using cron expressions
- **Automated Retention**: Schedule cleanup of old snapshots using restic forget
- **Auto-Restore**: Automatically restore new PVCs from existing backups
- **Manual Operations**: Trigger backups and restores via annotations
- **Multi-Repository Support**: Backup to multiple repositories with priority ordering
- **Workload Management**: Scale down workloads during backup/restore operations
- **PVC Selection**: Flexible PVC selection using label selectors and namespace filters
- **Restore from one PVC into completely different one**: You can annotate any PVC to restore from any other PVC.

### Common Operations (via Annotations)

Here are the most common manual operations you can perform by annotating your resources.

#### Triggering a Manual Backup of a Single PVC

```sh
kubectl annotate pvc my-pvc \
    backup.datarestor-operator.com/manual-backup='pre-upgrade-snapshot'
```

#### Triggering a Manual Backup for all PVCs in a Config

```sh
kubectl annotate config my-config \
    backup.datarestor-operator.com/manual-backup='my-manual-backup-run'
```

#### Triggering a Restore to a PVC

We can specify just a name of the backup to restore from.

```sh
kubectl annotate pvc my-pvc \
    backup.datarestor-operator.com/manual-restore='pre-upgrade-snapshot'
```

Or we can specify snapshot ID (restic's ID) to restore from.

```sh
kubectl annotate pvc my-pvc \
    backup.datarestor-operator.com/manual-restore='1234567890'
```

Or we can specify `latest`, `true`, or `now` to restore from the most recent snapshot (default for auto-restore).

```sh
kubectl annotate pvc my-pvc \
    backup.datarestor-operator.com/manual-restore='latest'
```

Or we can specify a specify repository (using priority or target) and different host (`{pvcNamespace}-{pvcName}`) to restore from (basically cloning existing PVC into different one).

```sh
kubectl annotate pvc my-pvc \
    backup.datarestor-operator.com/manual-restore='1#my-pvc-namespace-my-pvc-name#4dc109'
```

```sh
kubectl annotate pvc my-pvc \
    backup.datarestor-operator.com/manual-restore='s3:http://minio:9000/pg#myns-mypvc-name#now'
```

---

### Example Config

```yaml
apiVersion: backup.datarestor-operator.com/v1alpha1
kind: Config
metadata:
  name: my-backup-config
  namespace: backup-system
spec:
  repositories:
    # Primary backup to AWS S3
    - target: "s3:s3.amazonaws.com/my-bucket/backups"
      priority: 1
      backupSchedule: "0 2 * * *" # Daily at 2 AM
      forgotSchedule: "0 4 * * 0" # Weekly on Sunday at 4 AM
      forgetArgs: ["--keep-last", "10", "--keep-daily", "7"]
      env:
        - name: AWS_ACCESS_KEY_ID
          value: "your-access-key-here"
        - name: AWS_SECRET_ACCESS_KEY
          value: "your-secret-key-here"

    # Secondary backup to MinIO (self-hosted S3-compatible storage)
    - target: "s3:http://minio:9000/backup-bucket"
      priority: 2
      backupSchedule: "0 3 * * *" # Daily at 3 AM (1 hour after primary)
      forgotSchedule: "0 5 * * 0" # Weekly on Sunday at 5 AM
      forgetArgs: ["--keep-last", "5", "--keep-daily", "3"]
      env:
        - name: AWS_ACCESS_KEY_ID
          value: "minio-access-key"
        - name: AWS_SECRET_ACCESS_KEY
          value: "minio-secret-key"

  selectors:
    - matchLabels:
        app: "my-app"
    - matchNamespaces:
        - "default"
        - "production"
    - matchLabels:
        backupMeUp: "true"

  stopPods: true
  autoRestore: true
```

---

## Installation (under construction)

### Using Helm

```bash
# Add the Helm repository
helm repo add datarestor-operator https://sladg.github.io/datarestor-operator
helm repo update

# Install the operator
helm install datarestor-operator datarestor-operator/datarestor-operator \
  --namespace datarestor-operator-system \
  --create-namespace
```

## Helm Chart

The operator is available as a Helm chart that's automatically generated from the Kustomize manifests using [helmify](https://github.com/arttor/helmify). This ensures consistency and eliminates code duplication.

The Helm chart is generated in the `charts/datarestor-operator/` directory and automatically published to GitHub Pages when changes are pushed to the `master` branch.

## Roadmap

- [x] Add finalizers to workloads to prevent pod startup until an auto-restore is complete.
- [x] Automatically scale associated workloads (Deployments, etc.) up/down when `stopPods` is used.
- [x] Update RBAC to ensure least-privilege permissions.
- [x] Enable maintenance on repository - keep last N snapshots, prune old snapshots, verify snapshots.
- [x] Verify automatic-populate on newly created PVCs,
- [x] Verify manual restore on existing PVC,
- [x] Verify manual restore on new PVC (specify other PVC to restore from),
- [x] Fix CRD statuses to correctly match and update based on events.
- [x] Helm chart support with automatic generation using helmify
- [ ] Verify working with sqlite database with continuous writes.
- [ ] Allow for extended configuration of restic (compression, pruning, etc.)
- [ ] Verify working with PostgreSQL database with continuous writes.
- [ ] Support secrets refs in envs.
- [ ] Add support for snapshot verification.
- [ ] Improve how we find auto-restore snapshot. We should go one-by-one based on repository priority and see if backup exists. If not, we should move to next repository.
- [ ] Allow for per-selector and per-repository configuration of auto-restore and stopPods (forget as well?).
- [ ] (Future) Fix permissions for secrets (envs) in CRDs.
- [ ] (Future) Allow for snapshots of volumes instead of copy-paste of file system - quicker backups/restores.
- [ ] (Future) E2E tests for mid-operation deletions, we should correctly unlock and proceed without blocking anything.
- [ ] (Future) Scan Restic and in case of deletion / pruning, prune our Tasks that no longer refence backups in repository.
- [ ] (Future) Config to have information about size of backups/restores, duration of how long it took, etc. Also, include number of backups, successes
