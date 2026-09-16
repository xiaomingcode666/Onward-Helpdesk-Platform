# FND-003 environment inventory

This directory describes deployment instances, not the smaller service categories
inside a company's ticket form. Keep one `shared_integration` instance and one
`staging` / `production` pair for every delivery project.

Copy `inventory.example.json` into an operator-owned inventory and supply real
host IDs, HTTPS hostnames, Linux installation directories and company IDs.
Each delivery project has a distinct existing company (`tenant_id`); its staging
and production instances use the same company ID in their separate databases.
The shared integration instance uses its own company ID and synthetic test data.
This tool does not create company records or copy production data.

Integration and staging packages also contain `data-provenance.json`. The file
must state `synthetic` or `approved_sanitized` data, and must include a passed
verification. Approved sanitized data additionally records an approval reference,
approver and UTC approval time. `environmentctl` and deployment preflight reject
missing, invalid, unapproved or failed attestations before an environment is
started. Validate an attestation before any manual test-data import with:

```sh
python scripts/validate-test-data.py --manifest /path/data-provenance.json --environment staging
```

The gate records evidence; it does not claim to transform production data. A
data owner must run the approved sanitization process first and then replace the
generated synthetic attestation with the approved-sanitized record. Production
packages do not carry this non-production attestation.

```sh
python scripts/environmentctl.py validate --inventory /path/inventory.json
python scripts/environmentctl.py list --inventory /path/inventory.json
python scripts/environmentctl.py select --inventory /path/inventory.json --instance onward-knowledge-service-staging
python scripts/environmentctl.py render --inventory /path/inventory.json --output /path/new-packages
python -m unittest discover -s scripts/tests -p test_environmentctl.py -v
```

`render` also works on Windows. Its output paths describe the **Linux target**;
the package can be securely transferred to that target's `deploy_root`. Every
instance receives separate data, backup, state, metrics, configuration and secret
directories, a unique Compose project/network, nine loopback ports and fresh
database/storage/application credentials. Its image name is also unique, because
frontend builds include each instance's public URL. Protect the output as a secret; never
commit it. `.env` files are created with mode `0600` on POSIX. When transferring
from Windows, explicitly preserve or restore `0600` on `.env` and `0700` on secret
directories on the Linux host.

Runtime YAML, Alertmanager YAML and copied configuration bundles use `0644`, and
the mounted `project-config` directory uses `0755`, so the API's non-root user can
read them. These files must contain settings, placeholders and secret references
only; never put plaintext credentials in them. Real secret directories stay
restricted. Grant the API container UID the required read/traverse ACL or
ownership on its own secret files and directories; do not make them public.
The deployment configuration check runs as the actual API user and verifies
access before startup. The renderer does not relax secret-file permissions.

The inventory itself contains no credentials. `config_bundle` is either an
existing FND-002 export (relative to the inventory file or an absolute local
path), or an empty string. An empty value produces a `pending_config` package
with no fabricated version or digest. It cannot pass deployment preflight.
Provided exports must match the company's ID and target environment exactly;
the tool copies their bytes without converting development into production.
The existing FND-002 checker must still validate the full schema, digest, policy
and actual secret references before deployment. Supply those secret files in
`project-secrets/<tenant_id>/<environment>/`, choose an approved image release,
and run target-host preflight. Rendering by itself never claims deployability.

All input conflicts and existing output instances are checked before files are
created. Existing packages are never overwritten: rerunning cannot reset their
credentials. Use an explicit new output directory to prepare a different package;
configuration updates for an existing installation follow the FND-002 deployment
workflow and preserve its current infrastructure credentials.

For automation, `select` returns one JSON object with `instance_id`,
`deployment_project_id`, `environment`, `tenant_id`, `host_id`, `deploy_root`,
`public_url`, `files_url`, `port_base`, `ports`, `paths`, `config_digest` and
`config_status`. The generated `instance.json` has the same fields plus
`schema_version: 1`. Neither output includes passwords or the local bundle path.
`load_targets(inventory_path)` provides the equivalent Python interface and
additionally includes the resolved local `config_bundle` path for the renderer.

Port offsets from `port_base`: application `+0`, PostgreSQL `+1`, Redis `+2`,
Qdrant HTTP `+3`, Qdrant gRPC `+4`, MinIO `+5`, MinIO console `+6`, Prometheus
`+7`, Alertmanager `+8`. Instances sharing a host cannot overlap ports or
deployment directories. Public hostnames must be unique across the inventory.

Use `scripts/deploy-environment.py` to check an offline package or operate the
selected instance on its Linux host. It matches the package manifest, `.env`,
instance ID, host ID and exact deployment directory before invoking `rhdctl`.
`--action initialize-config --document <package>/project-config/initial.json
--tenant-name <name> --admin-username <login>
--admin-password-file <package>/project-secrets/<tenant_id>/<environment>/bootstrap-admin-password`
explicitly initializes a new database and its enterprise administrator. Prepare
the password file privately with mode `0600`; never put its contents in CLI
arguments. Alternatively, `--admin-user-id <id>` binds an existing standalone
account. Same-account retries never reset credentials; a matching existing bundle
can be reused to repair a missing first administrator without overwriting it.
`preflight`,
`install`, `doctor`, `backup`, `restore-drill` and `upgrade` reuse the existing
deployment workflow. `sync-config-digest` revalidates an existing configuration
after an interrupted initialization before updating its environment digest.

Managed backups include `.dump`, `.sha256`, `.meta` and `.config.json`; keep all
four files together. The configuration comes from the database's active version,
which may differ from a pending deployment file. Restore validates the archived
configuration and required secrets before replacing the database, restores the
matching bundle and env digest, and leaves the application stopped on failure.
Use the host `rhdctl restore` entry point with Python 3 and the release's Go
configuration checker. Secrets remain separately managed and are not archived.

See [FND-003 delivery and operation guide](../../work/FND-003-环境隔离与交付说明.md)
for complete commands, file ownership, existing-database migration boundaries and
executed validation. The root Compose and global GitHub deployment workflow are
the legacy single-instance route, not the environment inventory deployment path.
