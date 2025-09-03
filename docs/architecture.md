# Architecture

This document provides a high-level overview of the Custom Resource Definitions (CRDs) and controller logic for the AutoRestore Backup Operator.

## Custom Resource Definitions (CRDs)

The operator uses four primary CRDs to manage backup and restore operations.

### 1. BackupConfig

**Purpose**: Defines a backup policy, including what to back up, where to store it, how often to back it up, and how long to retain backups. It acts as the central orchestration resource.

**CRD Definition**: `api/v1alpha1/backupconfig_types.go`

#### Primary Fields (`kubectl get`)

These fields are displayed as columns in the `kubectl get backupconfigs` output.

| Column            | Type    | Description                                                 |
| ----------------- | ------- | ----------------------------------------------------------- |
| `PVCs`            | integer | The number of PVCs currently managed by this configuration. |
| `Targets`         | integer | The number of backup targets (repositories) configured.     |
| `Backup Success`  | integer | The total number of successful backup jobs.                 |
| `Backup Running`  | integer | The number of currently running backup jobs.                |
| `Backup Failed`   | integer | The total number of failed backup jobs.                     |
| `Restore Success` | integer | The total number of successful restore jobs.                |
| `Restore Running` | integer | The number of currently running restore jobs.               |
| `Restore Failed`  | integer | The total number of failed restore jobs.                    |
| `Age`             | date    | The age of the `BackupConfig` resource.                     |

#### Spec Details (`kubectl describe`)

These fields define the desired state of the `BackupConfig`.

| Field           | Type             | Description                                                                                                                                                                  |
| --------------- | ---------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `selectors`     | `[]Selector`     | A list of selectors to find PVCs. A PVC is selected if it matches _any_ of the selectors in this list (OR logic).                                                            |
| `backupTargets` | `[]BackupTarget` | A prioritized list of destinations where backups will be stored. Each target has its own Restic configuration, retention policy, and schedule.                               |
| `autoRestore`   | `bool`           | If true, automatically restores the latest backup to newly created PVCs that match one of the selectors. The first `BackupConfig` to claim the PVC will perform the restore. |

#### Status Details (`kubectl describe`)

These fields represent the observed state of the `BackupConfig`.

| Field                | Type                   | Description                                                                        |
| -------------------- | ---------------------- | ---------------------------------------------------------------------------------- |
| `managedPVCs`        | `[]string`             | A list of PVCs currently being managed and backed up by this configuration.        |
| `targets`            | `[]BackupTargetStatus` | Status for each individual backup target, including `lastBackup` and `nextBackup`. |
| `backupJobs`         | `JobStatistics`        | Aggregated statistics about the backup jobs (successful, running, failed, etc.).   |
| `restoreJobs`        | `JobStatistics`        | Aggregated statistics about the restore jobs.                                      |
| `lastRetentionCheck` | `metav1.Time`          | Timestamp of the last time the retention policy was applied.                       |
| `conditions`         | `[]metav1.Condition`   | The observed conditions of the `BackupConfig` resource.                            |

---

### 2. ResticRepository

**Purpose**: Represents a single Restic backup repository. It manages the lifecycle of the repository itself, including initialization and maintenance tasks like `check` and `prune`.

**CRD Definition**: `api/v1alpha1/resticrepository_types.go`

#### Primary Fields (`kubectl get`)

| Column       | Type    | Description                                                                        |
| ------------ | ------- | ---------------------------------------------------------------------------------- |
| `Repository` | string  | The URL of the Restic repository.                                                  |
| `Phase`      | string  | The current lifecycle phase of the repository (`Initializing`, `Ready`, `Failed`). |
| `Snapshots`  | integer | The total number of snapshots in the repository.                                   |
| `Size`       | string  | The total size of the repository.                                                  |
| `Age`        | date    | The age of the `ResticRepository` resource.                                        |

#### Spec Details (`kubectl describe`)

| Field                 | Type                            | Description                                                                                                                                                                                            |
| --------------------- | ------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `repository`          | `string`                        | The URL of the Restic repository (e.g., `s3:s3.amazonaws.com/bucket/path`).                                                                                                                            |
| `passwordSecretRef`   | `SecretKeySelector`             | A reference to the Kubernetes Secret containing the repository password.                                                                                                                               |
| `env`                 | `[]corev1.EnvVar`               | Environment variables required for repository access (e.g., cloud provider credentials).                                                                                                               |
| `additionalFlags`     | `[]string`                      | **[Advanced]** A list of additional flags to pass directly to the Restic backup command. Use with caution.                                                                                             |
| `maintenanceSchedule` | `RepositoryMaintenanceSchedule` | Cron schedules for running repository maintenance tasks (`check`, `prune`, `verification`). “Verification” refers to snapshot integrity checks (e.g., read-data subset) and can be resource-intensive. |

#### Status Details (`kubectl describe`)

| Field         | Type                 | Description                                                                                   |
| ------------- | -------------------- | --------------------------------------------------------------------------------------------- |
| `phase`       | `string`             | The current lifecycle phase of the repository (`Unknown`, `Initializing`, `Ready`, `Failed`). |
| `lastChecked` | `metav1.Time`        | Timestamp of the last successful repository integrity check (`restic check`).                 |
| `stats`       | `RepositoryStats`    | Statistics about the repository, such as total size and snapshot count.                       |
| `error`       | `string`             | A detailed error message if the repository is in a `Failed` state.                            |
| `conditions`  | `[]metav1.Condition` | The observed conditions of the `ResticRepository` resource.                                   |

---

### 3. ResticBackup

**Purpose**: Represents a single backup snapshot within a `ResticRepository`. It is created by a `BackupConfig` according to its schedule and corresponds to a point-in-time backup of a specific PVC.

**CRD Definition**: `api/v1alpha1/resticbackup_types.go`

#### Primary Fields (`kubectl get`)

| Column        | Type   | Description                                                                              |
| ------------- | ------ | ---------------------------------------------------------------------------------------- |
| `Backup Name` | string | The user-defined or generated name of the backup.                                        |
| `PVC`         | string | The name of the PVC that was backed up.                                                  |
| `Phase`       | string | The current lifecycle phase of the backup (`Pending`, `Running`, `Completed`, `Failed`). |
| `Snapshot ID` | string | The unique ID of the snapshot within the Restic repository.                              |
| `Size`        | string | The size of the backup data.                                                             |
| `Repository`  | string | The name of the `ResticRepository` CRD where this backup is stored.                      |
| `Duration`    | string | The total time taken for the backup job to complete.                                     |
| `Age`         | date   | The age of the `ResticBackup` resource.                                                  |

#### Spec Details (`kubectl describe`)

| Field           | Type                          | Description                                                                              |
| --------------- | ----------------------------- | ---------------------------------------------------------------------------------------- |
| `backupName`    | `string`                      | A human-readable name for the backup.                                                    |
| `pvcRef`        | `PVCReference`                | A reference to the PVC that was backed up, including its name and namespace.             |
| `repositoryRef` | `corev1.LocalObjectReference` | A reference to the `ResticRepository` CRD where this backup is stored.                   |
| `backupType`    | `string`                      | The type of backup (e.g., `scheduled`, `manual`).                                        |
| `snapshotID`    | `string`                      | The unique ID of the snapshot within the Restic repository. Populated by the controller. |
| `tags`          | `[]string`                    | Tags applied to the snapshot in the Restic repository.                                   |

#### Status Details (`kubectl describe`)

| Field            | Type                 | Description                                                                              |
| ---------------- | -------------------- | ---------------------------------------------------------------------------------------- |
| `phase`          | `string`             | The current lifecycle phase of the backup (`Pending`, `Running`, `Completed`, `Failed`). |
| `startTime`      | `metav1.Time`        | Timestamp of when the backup job started.                                                |
| `completionTime` | `metav1.Time`        | Timestamp of when the backup job finished.                                               |
| `size`           | `int64`              | The size of the backup data in bytes.                                                    |
| `lastVerified`   | `metav1.Time`        | Timestamp of the last time this snapshot's existence was verified.                       |
| `error`          | `string`             | A detailed error message if the backup is in a `Failed` state.                           |
| `duration`       | `string`             | The total time taken for the backup job to complete, in a human-readable format.         |
| `conditions`     | `[]metav1.Condition` | The observed conditions of the `ResticBackup` resource.                                  |

---

### 4. ResticRestore

**Purpose**: Represents a request to restore a PVC from a specific `ResticBackup`. It is a one-shot operation that runs a restore job and reports its status.

**CRD Definition**: `api/v1alpha1/resticrestore_types.go`

#### Primary Fields (`kubectl get`)

| Column       | Type   | Description                                                                               |
| ------------ | ------ | ----------------------------------------------------------------------------------------- |
| `Backup`     | string | The name of the `ResticBackup` to restore from.                                           |
| `Target PVC` | string | The name of the PVC to restore the data into.                                             |
| `Phase`      | string | The current lifecycle phase of the restore (`Pending`, `Running`, `Completed`, `Failed`). |
| `Duration`   | string | The total time taken for the restore job to complete.                                     |
| `Age`        | date   | The age of the `ResticRestore` resource.                                                  |

#### Spec Details (`kubectl describe`)

| Field                   | Type           | Description                                                                    |
| ----------------------- | -------------- | ------------------------------------------------------------------------------ |
| `backupName`            | `string`       | The name of the `ResticBackup` CRD to restore from.                            |
| `targetPVC`             | `string`       | The name of the PVC to restore the data into.                                  |
| `backupTarget`          | `BackupTarget` | The Restic repository configuration to use for the restore operation.          |
| `additionalRestoreArgs` | `[]string`     | Additional arguments to pass to the restic restore command.                    |
| `snapshotID`            | `string`       | Specific snapshot ID to restore from (optional, uses latest if not specified). |

#### Status Details (`kubectl describe`)

| Field            | Type                 | Description                                                                               |
| ---------------- | -------------------- | ----------------------------------------------------------------------------------------- |
| `phase`          | `string`             | The current lifecycle phase of the restore (`Pending`, `Running`, `Completed`, `Failed`). |
| `startTime`      | `metav1.Time`        | Timestamp of when the restore job started.                                                |
| `completionTime` | `metav1.Time`        | Timestamp of when the restore job finished.                                               |
| `error`          | `string`             | A detailed error message if the restore is in a `Failed` state.                           |
| `conditions`     | `[]metav1.Condition` | The observed conditions of the `ResticRestore` resource.                                  |
| `restoredSize`   | `int64`              | The total size of the data restored in bytes.                                             |

---

## Reconciliation Best Practices

To ensure robust and predictable behavior, all controllers in this operator adhere to the following best practices for their reconciliation logic.

### 1. Atomic Updates (Read-Modify-Write)

All updates to Kubernetes resources follow the "read-modify-write" pattern. The controller first reads the latest version of an object from the API server, calculates the required changes in memory, and then writes the entire modified object back. This pattern leverages Kubernetes' optimistic concurrency control (via the `resourceVersion` field) to prevent stale writes and ensure that changes are applied atomically. If another process modifies the object between the read and write, the write operation will fail, and the reconciliation will be automatically retried with the newest version of the object.

### 2. Separation of Spec and Status

The `.spec` and `.status` subresources of a Custom Resource are treated as being owned by different actors. The `.spec` (the desired state) is owned by the user, while the `.status` (the observed state) is owned by the controller.

To enforce this separation and prevent accidental overwrites of user-intent, controllers **must** use the dedicated `.Status().Update()` client method when updating only the status of a resource. General `.Update()` calls are reserved for changes that might affect the spec or metadata (like adding a finalizer).

### 3. Idempotency

The entire reconciliation loop is designed to be idempotent. This means that if a reconcile is triggered for a resource that is already in its desired state, the controller will perform its checks but will not make any unnecessary API calls or changes to the cluster. For example, before creating a backup `Job`, the controller first checks if a `Job` for that backup already exists and is active. This prevents the creation of duplicate resources and ensures the system remains stable even if it receives many redundant events for the same object.

---

## Ownership and Conflict Resolution

To prevent conflicts when multiple `BackupConfig` resources have selectors that match the same PVC, the operator uses an ownership-claiming mechanism. This ensures that a single, designated `BackupConfig` is responsible for managing any given PVC at one time.

This is achieved using a label on the PVC:

| Label                               | Value                | Description                                                                                                                          |
| ----------------------------------- | -------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `backup.autorestore.com/managed-by` | `<namespace>/<name>` | Indicates which `BackupConfig` has "claimed" this PVC. Once this label is set, other `BackupConfig` controllers will ignore the PVC. |

The first `BackupConfig` controller to reconcile a newly matched PVC will attempt to add this label to it. This atomic operation acts as a lock, preventing race conditions and ensuring that only one controller orchestrates backups and auto-restores for that PVC.

---

## Interaction via Annotations

To simplify manual operations, users can trigger backups and restores directly by annotating a `PersistentVolumeClaim` (PVC). The `BackupConfig` controller, which manages the PVC, will detect these annotations and orchestrate the corresponding action.

| Annotation                                      | Value                                                                    | Description                                                                                                                                                                                                                                                                                       |
| ----------------------------------------------- | ------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `backup.datarestor-operator.com/manual-backup`  | `"true"`, `"now"`, or `<custom-backup-name>`                             | Triggers a one-time backup. If the value is `"true"` or `"now"`, a default name is generated. Otherwise, the value is used as the name for the `ResticBackup` resource. This can be applied to a **PVC** or a **BackupConfig**. The operator removes the annotation after triggering the process. |
| `backup.datarestor-operator.com/manual-restore` | `"true"`, `"now"`, `"latest"`, `<snapshot-ID>`, or `<ResticBackup-Name>` | Triggers a restore to the annotated PVC. The value can be a specific Restic snapshot ID, the name of a `ResticBackup` resource, or `"latest"`, `"true"`, or `"now"` to restore the most recent snapshot. The operator creates a `ResticRestore` resource and then removes this annotation.        |

---

## Controller Logic

This section describes the behavior of each controller's `Reconcile` loop. All reconciliation steps described below adhere to the [Reconciliation Best Practices](#reconciliation-best-practices) outlined previously.

### 1. BackupConfig Controller

**Purpose**: To watch `BackupConfig` resources and orchestrate the entire backup lifecycle based on their specifications.

#### Reconciliation Triggers

The `Reconcile` loop is triggered by:

1.  Changes (creation, updates, deletion) to a `BackupConfig` resource.
2.  Changes to PVCs that are owned by a `BackupConfig` (via a watch). This includes the addition of the manual trigger annotations described above.
3.  Periodic requeuing for scheduled backups and retention policy checks.

#### Reconciliation Steps

On each reconciliation, the controller performs the following high-level steps:

1.  **Fetch `BackupConfig` Instance**: Retrieves the current state of the `BackupConfig` resource that triggered the event.

2.  **Handle Deletion**:

    - Checks if the `BackupConfig` is marked for deletion (i.e., has a `deletionTimestamp`).
    - If so, it runs the deletion logic, which includes removing its ownership label from any PVCs it manages before cleaning up child resources.

3.  **Discover and Claim PVCs**:

    - It discovers all PVCs across the cluster that match any of the `spec.selectors`.
    - For each matched PVC, it checks for the `backup.autorestore.com/managed-by` label.
      - If the label is **not present**, the controller attempts to "claim" the PVC by setting the label to its own name. The first controller to succeed wins.
      - If the label **is present** and matches the controller's name, it proceeds with managing the PVC.
      - If the label **is present** but points to a different `BackupConfig`, the controller ignores the PVC.
    - The list of successfully claimed PVCs is then updated in the `status.managedPVCs` field via a status-only update.

4.  **Handle Auto-Restore**:

    - For each newly claimed PVC, if `spec.autoRestore` is true, the controller initiates the auto-restore process.
    - **Safety Pre-flight**: Before creating a `ResticRestore` job, the controller finds any `Deployments` or `StatefulSets` that use this PVC and scales them to 0 replicas, recording their original replica counts. This prevents the workload's pods from starting up with an empty volume while the restore is in progress.
    - It then creates the `ResticRestore` resource to orchestrate the restore, defaulting to the highest-priority `BackupTarget` (lowest `priority` value) unless specified otherwise.

5.  **Handle Annotations**:

    - It checks the `BackupConfig` resource itself _and_ its managed PVCs for manual trigger annotations (`manual-backup` and `restore-from-backup`) and creates the necessary `ResticBackup` or `ResticRestore` resources if they are found.

6.  **Schedule Backups**:

    - The controller manages the backup schedule for its claimed PVCs. This logic iterates through each `BackupTarget` in the spec:
      - For each target, it checks its individual `schedule`.
      - It compares the schedule against the `lastBackup` time for that specific target (from `status.targets`).
      - **Concurrency Check**: Before creating a new backup, it checks if there is already a `ResticBackup` in a `Pending` or `Running` state for the given PVC and target. If one exists, it skips creation to enforce a maximum of one pending/active job.
      - If a backup is due and no other job is pending, it creates an idempotent `ResticBackup` resource, associating it with that target.
      - It updates the `nextBackup` and `lastBackup` timestamps for the specific target in the `.status` subresource.

7.  **Apply Retention Policy**:
    - The controller enforces the retention rules for backups related to its claimed PVCs. This process is per-target.

---

### 2. ResticRepository Controller

**Purpose**: To manage the lifecycle of a `ResticRepository` resource, ensuring the underlying Restic repository is initialized, healthy, and maintained.

#### Reconciliation Triggers

The `Reconcile` loop is triggered by:

1.  Changes (creation, updates, deletion) to a `ResticRepository` resource.
2.  Changes to child `Job` resources owned by the `ResticRepository`.
3.  Periodic requeuing for scheduled maintenance tasks.

#### Reconciliation Steps

The controller uses a phase-based approach to manage the repository's lifecycle. The `status.phase` field dictates the actions taken during reconciliation.

1.  **Fetch `ResticRepository` Instance**: Retrieves the current state of the resource.

2.  **Handle Deletion (`HandleRepoDeletion`)**:

    - If the resource has a `deletionTimestamp`, this logic is triggered.
    - It's responsible for running cleanup jobs to remove the repository data from the backend storage before removing the finalizer and allowing the CRD to be deleted.

3.  **Phase: Initializing (`HandleRepoInitialization`)**:

    - This is the entry point for newly created `ResticRepository` resources. The controller will always attempt to initialize the repository if it cannot be detected.
    - It creates a Kubernetes `Job` to run `restic init` on the specified repository backend.
    - It then transitions the phase to monitor the initialization job.

4.  **Phase: Initializing Status (`HandleRepoInitializationStatus`)**:

    - Monitors the initialization job created in the previous step.
    - If the job succeeds, it updates the `.status` subresource and transitions the phase to `Ready`.
    - If the job fails, it updates the `.status` subresource and transitions the phase to `Failed`.

5.  **Phase: Ready (`HandleRepoMaintenance`)**:

    - This is the normal, healthy state for a repository.
    - It checks the `maintenanceSchedule` to see if it's time to run a `check`, `prune`, or `verification` job (verification may perform snapshot data checks and can be resource-intensive).
    - If a maintenance task is due, it creates the corresponding Kubernetes `Job` idempotently.

6.  **Phase: Failed (`HandleRepoFailed`)**:

    - This is the terminal state when an unrecoverable error occurs (e.g., initialization fails).
    - The controller takes no further action and relies on manual intervention to resolve the issue with the repository backend.

7.  **Status Updates (`UpdateRepoStatus`)**: Throughout the process, the controller continuously updates the `.status` subresource of the `ResticRepository` resource, reflecting the current phase, job references, statistics, and any errors.

---

### 3. ResticBackup Controller

**Purpose**: To execute a single backup operation for a specific PVC and manage the lifecycle of the resulting snapshot as represented by the `ResticBackup` CRD.

#### Reconciliation Triggers

The `Reconcile` loop is triggered by:

1.  Changes (creation, updates, deletion) to a `ResticBackup` resource.
2.  Changes to a child `Job` resource owned by the `ResticBackup`.

#### Reconciliation Steps

Like the repository controller, this controller uses a phase-based approach to manage the backup lifecycle.

1.  **Fetch `ResticBackup` Instance**: Retrieves the current state of the resource.

2.  **Handle Deletion (`HandleBackupDeletion`)**:

    - If the resource has a `deletionTimestamp`, this logic is triggered.
    - It creates a Kubernetes Job to run `restic forget` and `restic prune` to remove the specific snapshot from the repository backend.
    - Once the job is complete, it removes the finalizer, allowing the `ResticBackup` CRD to be deleted.

3.  **Phase: Pending (`HandleBackupPending`)**:

    - This is the entry point for newly created `ResticBackup` resources.
    - **Concurrency Check**: Before creating a Kubernetes `Job`, the controller first lists all active `Jobs` in the cluster and checks if another backup or restore job is already running for the same PVC. If an active job is found, the controller will requeue and wait, ensuring only one job operates on a PVC at a time.
    - It performs pre-flight checks, such as ensuring the referenced `ResticRepository` is in a `Ready` state.
    - It creates the main Kubernetes `Job` to run the `restic backup` command.
    - It transitions the phase to `Running`.

4.  **Phase: Running (`HandleBackupRunning`)**:

    - Monitors the backup `Job` created in the `Pending` phase.
    - It uses a pipeline to check the job's status:
      - **If Failed**: It updates the `.status` subresource with the error message and transitions the phase to `Failed`.
      - **If Completed**: It parses the job's output to get the snapshot ID, updates the `ResticBackup` spec and status accordingly, and transitions the phase to `Completed`.
      - **If Still Running**: It requeues the reconciliation to check again later.

5.  **Phase: Completed**:

    - Final successful state. Ensures the `.status` subresource is updated correctly. No further transitions occur after completion.

6.  **Phase: Failed (`HandleBackupFailed`)**:
    - This is a terminal state indicating the backup job failed.
    - The controller logs the error in the status and takes no further action. Manual intervention (e.g., deleting the `ResticBackup` CRD to retry) is required.

---

### 4. ResticRestore Controller

**Purpose**: To execute a single restore operation from a `ResticBackup` to a target PVC.

#### Reconciliation Triggers

The `Reconcile` loop is triggered by:

1.  Changes (creation, updates, deletion) to a `ResticRestore` resource.
2.  Changes to a child `Job` resource owned by the `ResticRestore`.

#### Reconciliation Steps

This controller also uses a phase-based approach to manage the one-shot restore operation.

1.  **Fetch `ResticRestore` Instance**: Retrieves the current state of the resource.

2.  **Handle Deletion (`HandleResticRestoreDeletion`)**:

    - If the resource has a `deletionTimestamp`, it cleans up the Kubernetes Job created for the restore and then removes its own finalizer, allowing the CRD to be deleted.

3.  **Phase: Pending (`HandleRestorePending`)**:

    - This is the entry point for newly created `ResticRestore` resources.
    - **Concurrency Check**: Similar to the backup controller, it first checks if another backup or restore job is already running for the `targetPVC`. If so, it waits and requeues.
    - It performs pre-flight checks:
      - Validates that the referenced `ResticBackup` exists.
      - Checks if the `targetPVC` exists.
    - Once validations pass, it creates a Kubernetes Job to run the `restic restore` command.
    - It transitions the phase to `Running`.

4.  **Phase: Running (`HandleRestoreRunning`)**:

    - Monitors the restore `Job` created in the `Pending` phase.
    - It uses a pipeline to check the job's status:
      - **If Failed**: It updates the `.status` subresource with the error message and transitions the phase to `Failed`.
      - **If Completed**: It updates the `.status` subresource, records the completion time, and transitions the phase to `Completed`.
      - **If Still Running**: It requeues the reconciliation to check again later.

5.  **Phase: Completed / Failed**:
    - These are terminal states. The controller takes no further action on the restore itself.
    - **Workload Unpause**: As a final step (whether the restore succeeded or failed), the controller scales the associated `Deployments` and `StatefulSets` back to their original replica counts, allowing pods to be created and to mount the now-restored volume.
    - The `ResticRestore` resource remains as a record of the operation until it is manually deleted.

---

## Scheduling and Requeuing

The controllers use a combination of watches and timed requeues to ensure the cluster's state eventually converges with the desired state defined by the CRDs.

### Watch-Based Reconciliation

- **Primary Trigger**: The primary trigger for any controller is a change (Create, Update, Delete event) to the CRD it manages. For example, creating a `ResticBackup` CRD immediately triggers the `ResticBackup` controller.
- **Secondary Triggers**: Controllers also watch related resources:
  - `BackupConfig` controller watches `PVCs` and workloads (`Deployments`, `StatefulSets`) that reference matched PVCs. A change to a matched PVC will trigger a `BackupConfig` reconciliation.
  - `ResticRepository`, `ResticBackup`, and `ResticRestore` controllers watch the `Jobs` they create. A change in a job's status (e.g., from `Running` to `Completed`) triggers the parent controller.

### Timed Requeues

In addition to watches, controllers request a reconciliation at a future time (`requeueAfter`). This is crucial for periodic tasks and for handling states where a watch event is not expected.

- **`BackupConfig` Controller**:

  - **Scheduled Backups**: The controller calculates the time of the next backup based on the cron schedule. It then requests a requeue for that specific time to ensure the backup job is created punctually.
  - **Retention Policy**: Similarly, it calculates the next time the retention policy should be run (e.g., daily at 3 AM) and requeues itself accordingly.
  - **Default Interval**: A default requeue interval (`constants.DefaultReconcileInterval`) is used as a fallback to ensure the controller periodically re-evaluates its state even if no events occur.

- **`ResticRepository` Controller**:

  - **Maintenance**: It calculates the next maintenance time based on the cron schedules in `spec.maintenanceSchedule` and requeues itself for that time.
  - **Job Monitoring**: When a maintenance job is in progress, it requeues with a short interval (e.g., 30 seconds) to poll the job's status until it completes.

- **`ResticBackup` & `ResticRestore` Controllers**:
  - **Job Monitoring**: When a backup or restore job is `Running`, the controller requeues with a short interval (e.g., 30 seconds) to poll the job's status. This is necessary because job completion is the primary event that moves the CRD to its next lifecycle phase.
  - **Failed State**: `Failed` is a terminal state. No automatic retries are performed. Users can manually initiate a new backup/restore if desired.

---

## Example Configuration

For a complete, commented example of a `BackupConfig` resource, please see the [Example Configurations](./examples.md) document.

---

## Additional Notes

- Size fields in status (e.g., `ResticBackup.status.size`, `ResticRepository.status.stats.totalSize`) are expressed in bytes. The corresponding `kubectl get` columns render a human-readable string (e.g., GiB).

- Manual backup annotation naming: when `backup.autorestore.com/manual-backup` is set to `"true"` or `"now"`, the operator generates a name in the form `manual-<pvc-name>-<yyyyMMdd-HHmmss>`.

- Retention policy (per-target): Each `BackupTarget` may define retention rules such as `keepLast`, `keepHourly`, `keepDaily`, `keepWeekly`, `keepMonthly`, or `keepWithin` (duration). The controller applies these rules per target during retention.

- Restore behavior: Use `additionalRestoreArgs` to control restore behavior (e.g., `--exclude` patterns, `--include` patterns). It is recommended to pause workloads by scaling them to 0 during restore to avoid data races or application-level corruption.

---

## Disaster Recovery (DR) Behavior

- Bootstrap flow: In DR scenarios where the cluster has no prior CRs, applying a valid `BackupConfig` will lead the operator to reconcile and (re)create any required `ResticRepository` resources. If the underlying repository already exists on disk/cloud, initialization will be treated idempotently: no failure is raised solely because the repository already exists.

- Existing data: The operator assumes the repository may contain existing snapshots. This is fully supported; backups and restores proceed using the existing data.

- Deletion semantics: The operator does not delete repository data from disk/cloud except when explicitly scheduled and executed by retention/prune policies. Retention rules (e.g., keep daily/hourly/weekly/monthly/within) determine when `restic forget` and `restic prune` are run. Outside of retention, repository contents are not removed.

### DR Scheduling Policy

- Catch-up behavior: After a DR bootstrap, the operator does not attempt to create "missed" backups immediately. Instead, it waits until the next scheduled cron window for each `BackupTarget` to avoid stressing the cluster while it is resource constrained.

- Forcing immediate backups (optional): If you need to run a backup immediately in DR, apply the manual backup annotation to a PVC or the `BackupConfig`:

  - PVC-level (single backup): `backup.autorestore.com/manual-backup: "now"`
  - Config-level (all managed PVCs): `backup.autorestore.com/manual-backup: "<name-or-now>"`
