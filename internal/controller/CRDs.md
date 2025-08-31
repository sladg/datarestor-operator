## CRDs

**TODO**: Align implementation with this documentation. Key changes needed:

- [x] Merge BackupJob + ResticBackup into single ResticBackup CRD
- [x] Rename RestoreJob → ResticRestore for consistency
- [x] Move snapshot verification from ResticBackup to ResticRepository controller
- [x] Make BackupConfig retention policy run daily (not every reconcile)
- [x] Update status phases to match documented values
- [x] Add missing fields (Backup Name, etc.)
- [x] When there is a backup/restore in progress, we should not be able to delete the PVC.
- [ ] we should start a batchJob and deal with the output of that job next time we reconcile. We should not be blocking the loop.
- [ ] allow to restore PVC from backup of different PVC
- [ ] restore should work even for blank repository/restic CRDs. In case of DR, we should be able to find stuff on disk and populate it to k8s
- [ ] for given restore/backup, use a pod instead of creating job for every restic command one by one, how can this be optimized? confirm first
- [ ] in case of restic errors, our backup jobs are created over and over again for single manual trigger, this should not happen

---

### ResticRepository

**Purpose**: Manages restic repository lifecycle and maintenance

**Key Fields:**

- Repository URL (e.g., s3://bucket/path, local:/path)
- Authentication credentials (password, env vars)
- Maintenance schedule (check/prune cron expressions)
- Auto-initialization flag

**Status Phases**: Unknown → Initializing → Ready → Failed

**Controller Responsibilities:**

- Initialize repository on creation (if needed)
- Daily repository integrity checks (`restic check`)
- Periodic repository pruning (`restic prune`)
- **Daily verification of ALL snapshots in repository**
- Delete repository data on CRD deletion (with finalizer)
- Track repository statistics (size, snapshot count, etc.)

**Ownership**: Created and owned by BackupConfig

---

### ResticBackup (merged BackupJob + ResticBackup)

**Purpose**: Represents both backup execution AND resulting snapshot

**Key Fields:**

- Backup Name (human-readable identifier)
- Snapshot ID (restic's unique identifier)
- PVC reference (what was backed up)
- Repository reference (which ResticRepository)
- Backup metadata (size, file count, paths, tags)
- Job execution details (start/completion times, job reference)

**Status Phases**: Pending → Running → Completed → Ready → Failed

- Pending: Scheduled but not yet started
- Running: Kubernetes Job executing backup
- Completed: Backup finished successfully, snapshot created
- Ready: Snapshot verified to exist in repository
- Failed: Job failed or snapshot verification failed

**Controller Responsibilities:**

- Execute backup via Kubernetes Job
- Track snapshot metadata and verification status
- Handle snapshot deletion via finalizer (`restic forget <id> --prune`)
- Self-cleanup based on retention policy

**Ownership**: Created and owned by BackupConfig
**Lifecycle**: Created by BackupConfig → Executes backup → Verified by ResticRepository → Deleted by retention policy

---

### BackupConfig

**Purpose**: Defines backup policies and orchestrates operations for matching PVCs

**Key Fields:**

- PVC selector (label selectors, namespaces)
- Backup targets (repositories, priorities)
- Backup schedule (cron expression)
- Retention policy (keep-last, keep-daily, keep-within, etc.)
- Auto-restore settings

**Controller Responsibilities:**

- Create ResticRepository CRDs for each backup target
- Schedule ResticBackup creation based on cron schedule
- **Daily retention policy enforcement** (not every reconcile!)
- Auto-restore orchestration for new PVCs
- PVC discovery and management

**Scheduling:**

- Backup scheduling: Based on spec.schedule cron
- Retention checks: Daily at configured time (e.g., 3 AM)
- Status updates: Every reconcile loop

**Retention Process:**
Daily checks for Restic's snapshots with `forget --dry-run` command. If snapshots are identified as expired/obsolete, corresponding ResticBackup CRDs are deleted (which triggers snapshot deletion via finalizer).

**Ownership**: Top-level resource (no owner)

---

### ResticRestore (renamed from RestoreJob)

**Purpose**: Performs restore operations from backup snapshots

**Key Fields:**

- PVC reference (target for restore)
- Backup ID or ResticBackup reference
- Restore type (manual/automated)
- Overwrite settings

**Status Phases**: Pending → Running → Completed → Failed

- Pending: Validation and preparation phase
- Running: Kubernetes Job executing restore
- Completed: Restore finished successfully
- Failed: Restore failed with error details

**Controller Responsibilities:**

- **Validate backup exists** via ResticBackup CRD (not direct restic calls)
- Coordinate with existing workloads (scaling, finalizers)
- Create and monitor Kubernetes Jobs for restore execution
- Handle pre/post restore resource management

**Ownership**: Created manually or by BackupConfig (auto-restore)
**Lifecycle**: One-time execution, not re-scheduled on failure
