# AutoRestore Backup Operator

## 🚧 UNDER CONSTRUCTION 🚧

---

An operator for automated PVC backup and restore with Restic.
A cheap, self-host-friendly operator that copies your volumes data on a schedule and populates them back automagically whenever you rebuild the cluster and your volumes are gone.

### Why?

Running a K8s cluster at home is fun — losing/restoring/rebuilding it isn’t. **AutoRestore** keeps rolling backups of your volumes in a Restic repo (S3, MinIO, USB drive… whatever).

- Configure once with a `BackupConfig`, then forget about it.
- New PVC appears? It’s backed up automatically.
- Cluster goes boom? Re-deploy the operator and it re-hydrates your volumes — no wall of YAML, no `kubectl cp`, no manual intervention.

Perfect for disaster recovery, painless migrations, simple rollbacks on failed upgrades.

## User Guide

For a complete guide to the operator's features, Custom Resource Definitions, and controller logic, please see the **[Architecture Document](./docs/architecture.md)**.

For concrete, copy-pasteable examples of the CRDs, please see the **[Examples Document](./docs/examples.md)**.

### Common Operations (via Annotations)

Here are the most common manual operations you can perform by annotating your resources.

#### Triggering a Manual Backup of a Single PVC

```sh
# This triggers a backup of 'my-pvc' and names the backup 'pre-upgrade-snapshot'.
kubectl annotate pvc my-pvc backup.datarestor-operator.com/manual-backup='pre-upgrade-snapshot'
```

#### Triggering a Manual Backup for all PVCs in a BackupConfig

```sh
# This triggers backups for all PVCs managed by 'my-backup-config'
# and names each backup 'my-manual-backup-run'.
kubectl annotate backupconfig my-backup-config backup.datarestor-operator.com/manual-backup='my-manual-backup-run'
```

#### Triggering a Restore to a PVC

```sh
# This triggers a restore to 'my-pvc' from the backup named 'pre-upgrade-snapshot'.
kubectl annotate pvc my-pvc backup.datarestor-operator.com/restore-from-backup='pre-upgrade-snapshot'
```

---

### Future Work

- [x] Add finalizers to workloads to prevent pod startup until an auto-restore is complete.
- [ ] Automatically scale associated workloads (Deployments, etc.) up/down when `stopPods` is used.
- [ ] Update RBAC to ensure least-privilege permissions.
- [ ] Enable maintenance on repository - keep last N snapshots, prune old snapshots, verify snapshots.
- [ ] Add support for snapshot verification.
- [ ] Verify automatic-populate on newly created PVCs,
- [ ] Verify manual restore on existing PVC,
- [ ] Verify manual restore on new PVC (specify other PVC to restore from),
- [ ] Allow for passing args to backups, restores, etc.

---

## Quick Start

```bash
# Install
make install && make deploy

# Or run locally
make run
```
