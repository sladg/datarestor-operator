Operator for k8s?

it would do:

- snapshot volumes from time to time and save to S3,
- on newly created stuff, it would check if volume is populated (PVC should be new and unique) and restore from snapshot,
- finalizers so that services wait for restoration?
- it should be configurable by labels on PV/PVC?

Databases? Sqlite? Might be problematic with data integrity.

Ability to turn the pod off for the snapshot to ensure data integrity?

Automatic in-place restore from remote data. This way, I can migrate and DR any time. I can start the cluster elsewhere and just point gitops to s3 and will restore automagically.

Only backup PVC if pod is running and is green?

Ability to specify amount of snapshots to keep? If I upgrade and something breaks, I want to easily rollback.

Allow for primary and secondary target? Aka. multiple targets with priority, if priority 1 is not present / does not have data, check priority 2, etc. …. this way we can sync with NFS every hour and sync to S3 every day or such. In case of recovery, NFS is prioritized, in case of DR S3 is used.

Longhorn does not support in-place restore from backup/snapshot, restoration is done into new volume and new PVC.

Use restic to have plain data for possibility of manual intervention?
