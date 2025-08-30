# PVC Backup Operator

## User notes

Manual backup:

```sh
kubectl annotate backupconfig my-backup backup.autorestore-backup-operator.com/manual-restore=<snapshot-id> --overwrite
```

Manual restore:

```sh
kubectl annotate backupconfig my-backup backup.autorestore-backup-operator.com/manual-restore=latest --overwrite
```

```sh
kubectl annotate backupconfig my-backup backup.autorestore-backup-operator.com/manual-restore=<snapshot-id> --overwrite
```

We are allowing for multiple targets. Automated restore prefers latest cluster-known backup. If not found, it will query restic directly and use latest backup. If not found, it will not restore anything - case of newly created PVCs.

### TODO

- [ ] Allow for file-level backups instead of snapshots.
- [ ] Restore resources into original count after restore/backup.
- [ ] Pass 1:1 envs and params to restic.
- [ ] Update RBAC to include least possible permissions.
- [ ] Allow for password to be passed in as a secret.
- [ ] Make backup schedule per-target.
- [ ] Allow for passing NFS mount options.

---

Kubernetes operator for automated PVC backup/restore with Restic. Simple, secure, and efficient.

## Quick Start

```bash
# Install
make install && make deploy

# Or run locally
make run
```

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
kubectl get backupconfig db-backup -o yaml

# View logs
kubectl logs -n autorestore-backup-operator-system deployment/autorestore-backup-operator-controller-manager
```

## Need Help?

- Check [Restic docs](https://restic.readthedocs.io/)
- Look at sample configs in `config/samples/`
- Open an issue on GitHub
