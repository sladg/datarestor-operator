# AutoRestore Backup Operator

## 🚧 UNDER CONSTRUCTION 🚧

---

An operator for automated PVC backup and restore with Restic.
A cheap, self-host-friendly operator that copies your volumes data on a schedule and populates them back automagically whenever you rebuild the cluster and your volumes are gone.

### Why?

Running a K8s cluster at home is fun — losing/restoring/rebuilding it isn’t. **AutoRestore** keeps rolling backups of your volumes in a Restic repo (S3, MinIO, USB drive… whatever).

- Configure once with a `Config`, then forget about it.
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
kubectl annotate pvc my-pvc backup.datarestor-operator.com/manual-backup='pre-upgrade-snapshot'
```

#### Triggering a Manual Backup for all PVCs in a Config

```sh
kubectl annotate config my-config backup.datarestor-operator.com/manual-backup='my-manual-backup-run'
```

#### Triggering a Restore to a PVC

We can specify just a name of the backup to restore from.

```sh
kubectl annotate pvc my-pvc backup.datarestor-operator.com/restore-from-backup='pre-upgrade-snapshot'
```

Or we can specify snapshot ID (restic's ID) to restore from.

```sh
kubectl annotate pvc my-pvc backup.datarestor-operator.com/restore-from-backup='1234567890'
```

Or we can specify `latest`, `true`, or `now` to restore from the most recent snapshot (default for auto-restore).

```sh
kubectl annotate pvc my-pvc backup.datarestor-operator.com/restore-from-backup='latest'
```

Or we can specify a specify repository (using priority or target) and different host (`{pvcNamespace}-{pvcName}`) to restore from (basically cloning existing PVC into different one).

```sh
kubectl annotate pvc my-pvc backup.datarestor-operator.com/restore-from-backup='1#my-pvc-namespace-my-pvc-name#4dc109'
```

```sh
kubectl annotate pvc my-pvc backup.datarestor-operator.com/restore-from-backup='s3:http://minio.local:9000/postgres-backups#my-pvc-namespace-my-pvc-name#now'
```

---

### Future Work

- [x] Add finalizers to workloads to prevent pod startup until an auto-restore is complete.
- [ ] Automatically scale associated workloads (Deployments, etc.) up/down when `stopPods` is used.
- [x] Update RBAC to ensure least-privilege permissions.
- [ ] Enable maintenance on repository - keep last N snapshots, prune old snapshots, verify snapshots.
- [ ] Add support for snapshot verification.
- [ ] Verify automatic-populate on newly created PVCs,
- [x] Verify manual restore on existing PVC,
- [ ] Verify manual restore on new PVC (specify other PVC to restore from),
- [ ] Allow for passing args to backups, restores, etc.
- [ ] Improve matching of repositories and checking for snapshots when restoring.
- [ ] Use restic's JSON output for improved matching of snapshots from annotation and auto-restore.
- [ ] Verify working with sqlite database with continuous writes.
- [ ] Fix CRD statuses to correctly match and update based on events.
- [ ] (Future) Fix permissions for secrets (envs) in CRDs.
- [ ] (Future) Allow for snapshots of volumes so quicker backups/restores.
- [ ] (Future) E2E tests for mid-operation deletions, we should correctly unlock and proceed without blocking anything.

---

## Quick Start

```bash
# Install
make install && make deploy

# Or run locally
make run
```
