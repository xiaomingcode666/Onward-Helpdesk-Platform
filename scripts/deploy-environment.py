#!/usr/bin/env python3
"""Inspect a generated FND-003 package or operate exactly one installed instance."""

from __future__ import annotations

import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

from environmentctl import (
    InventoryError, PORT_KEYS, ROOT, deployment_root, identifier,
    positive_int, public_origin, read_bundle, read_json, require,
)
from data_provenance import ProvenanceError, read as read_data_provenance


ACTIONS = ("check", "initialize-config", "sync-config-digest", "preflight", "install", "upgrade", "status", "doctor", "backup", "restore-drill")
PATH_SUFFIXES = {
    "RHD_DATA_DIR": "/volumes", "RHD_BACKUP_DIR": "/backups",
    "RHD_STATE_DIR": "/state", "RHD_METRICS_DIR": "/metrics",
    "RHD_CONFIG_FILE": "/remotehelpdesk.yaml",
    "RHD_PROJECT_CONFIG_DIR": "/project-config",
    "RHD_PROJECT_CONFIG_FILE": "/project-config/current.json",
    "RHD_PROJECT_SECRET_DIR": "/project-secrets",
    "RHD_TEST_DATA_PROVENANCE_FILE": "/data-provenance.json",
    "RHD_ALERTMANAGER_CONFIG_FILE": "/alertmanager.yml",
    "RHD_RESTORE_DATA_DIR": "/backups",
}
BIND_KEYS = (
    "APP_BIND_IP", "POSTGRES_BIND_IP", "REDIS_BIND_IP", "QDRANT_BIND_IP",
    "MINIO_BIND_IP", "MINIO_CONSOLE_BIND_IP", "PROMETHEUS_BIND_IP", "ALERTMANAGER_BIND_IP",
)


def read_environment(path: Path) -> dict[str, str]:
    require(path.is_file() and not path.is_symlink(), "A regular package .env file is required")
    require(path.stat().st_size <= 128 * 1024, "Environment file is too large")
    values = {}
    for line in path.read_text(encoding="utf-8-sig").splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        key, separator, value = line.partition("=")
        require(bool(separator) and re.fullmatch(r"[A-Z][A-Z0-9_]*", key) is not None,
                "Environment file must contain plain KEY=value assignments")
        require(key not in values, "Environment file contains a duplicate assignment")
        if len(value) >= 2 and value[0] in "\"'" and value[-1] == value[0]:
            value = value[1:-1]
        require(not any(c in value for c in "$`\x00"),
                "Environment file cannot contain substitutions; use plain values")
        values[key] = value
    return values


def inspect_package(package: Path, instance: str, host: str, allow_digest_sync: bool = False) -> tuple[dict, dict]:
    require(package.is_dir() and not package.is_symlink(), "Package must be a regular directory")
    manifest_file = package / "instance.json"
    require(not manifest_file.is_symlink(), "Instance manifest cannot be a symbolic link")
    manifest, _ = read_json(manifest_file, 128 * 1024)
    require(isinstance(manifest, dict) and type(manifest.get("schema_version")) is int and
            manifest["schema_version"] == 1, "Unsupported instance manifest")
    require(manifest.get("instance_id") == instance, "Selected instance does not match this package")
    require(identifier(manifest.get("host_id"), "host_id") == host,
            "Selected host does not match this instance")
    project = identifier(manifest.get("deployment_project_id"), "deployment_project_id")
    environment = manifest.get("environment")
    require(environment in ("integration", "staging", "production"), "Unsupported deployment environment")
    if environment == "integration":
        require(project == "daypop-shared" and instance == "onward-shared-integration",
                "Integration must use the shared instance identity")
    else:
        require(project not in ("daypop-shared", "shared") and instance == f"onward-{project}-{environment}",
                "Project instance identity is inconsistent")
    tenant = positive_int(manifest.get("tenant_id"), "tenant_id")
    root = deployment_root(manifest.get("deploy_root"))
    base = positive_int(manifest.get("port_base"), "port_base", 65535 - len(PORT_KEYS) + 1)
    require(base >= 1024, "Instance ports must be unprivileged")
    ports = {key: base + offset for offset, key in enumerate(PORT_KEYS)}
    paths = {key: root + suffix for key, suffix in PATH_SUFFIXES.items()}
    require(manifest.get("ports") == ports and manifest.get("paths") == paths,
            "Manifest ports or directories do not match the isolated instance layout")
    public, public_host = public_origin(manifest.get("public_url"), "public_url")
    files, files_host = public_origin(manifest.get("files_url"), "files_url")
    require(public_host != files_host, "Application and files must have separate hostnames")
    env = read_environment(package / ".env")
    expected = {
        "RHD_INSTANCE_ID": instance, "RHD_DEPLOYMENT_PROJECT_ID": project,
        "RHD_PROJECT_NAME": instance, "RHD_NETWORK_NAME": instance + "_1panel",
        "RHD_PROJECT_ENVIRONMENT": environment, "RHD_PROJECT_CONFIG_TENANT_ID": str(tenant),
        "RHD_PROJECT_CONFIG_REQUIRED": "1", "RHD_PUBLIC_URL": public,
        "MINIO_PUBLIC_ENDPOINT": files, "IMAGE_NAME": instance,
        **paths, **{key: str(value) for key, value in ports.items()},
        **{key: "127.0.0.1" for key in BIND_KEYS},
    }
    for key, value in expected.items():
        require(env.get(key) == value, f"{key} differs from the selected instance manifest")
    # Check copied package files before allowing any Docker invocation. FND-002
    # preflight performs the authoritative schema/policy/secret validation later.
    for relative in ("remotehelpdesk.yaml", "project-config", "project-secrets"):
        item = package / relative
        require(item.exists() and item.resolve().is_relative_to(package.resolve()) and not item.is_symlink(),
                "Package configuration and secret paths must stay inside the instance directory")
    provenance_summary = None
    if environment in ("integration", "staging"):
        provenance = package / "data-provenance.json"
        try:
            provenance_summary = read_data_provenance(provenance, environment)
        except (ProvenanceError, OSError) as exc:
            raise InventoryError(f"non-production data provenance check failed: {exc}") from exc
    bundle_path = package / "project-config/current.json"
    if bundle_path.exists():
        require(not bundle_path.is_symlink(), "Project configuration cannot be a symbolic link")
        bundle, _ = read_bundle(bundle_path, tenant, environment)
        matches = bundle["digest"] == env.get("RHD_PROJECT_CONFIG_DIGEST")
        require(matches or allow_digest_sync,
                "Configuration bundle and .env digest differ; install the matching FND-002 export")
        config_status = "pending_preflight" if matches else "pending_digest_sync"
    else:
        require(not env.get("RHD_PROJECT_CONFIG_DIGEST"), "Pinned configuration bundle is missing")
        config_status = "pending_config"
    return {"instance_id": instance, "deployment_project_id": project, "environment": environment,
            "tenant_id": tenant, "host_id": host, "deploy_root": root,
            "public_url": public, "config_status": config_status,
            "data_provenance": provenance_summary}, env


def operate(package: Path, instance: str, host: str, action: str,
            document: Path | None = None, tenant_name: str | None = None,
            admin_user_id: int | None = None, admin_username: str | None = None,
            admin_password_file: Path | None = None) -> int:
    require(action in ACTIONS, "Unsupported operation")
    report, env = inspect_package(package, instance, host, allow_digest_sync=action == "sync-config-digest")
    initialize_args = []
    if action == "initialize-config":
        require(document is not None and document.is_file() and not document.is_symlink() and
                document.resolve().parent == (package / "project-config").resolve(),
                "Initial document must be a regular JSON file directly inside this instance's project-config directory")
        require(isinstance(tenant_name, str) and 0 < len(tenant_name.strip()) <= 200 and
                not any(ord(c) < 32 for c in tenant_name), "Supply a company display name for explicit initialization")
        initialize_args = ["--document", str(document.resolve()), "--tenant-name", tenant_name.strip()]
        if admin_user_id is not None:
            positive_int(admin_user_id, "admin_user_id")
            require(admin_username is None and admin_password_file is None,
                    "Choose an existing administrator ID or a new username and password file")
            initialize_args += ["--admin-user-id", str(admin_user_id)]
        elif admin_username is not None or admin_password_file is not None:
            require(isinstance(admin_username, str) and 0 < len(admin_username) <= 100 and
                    admin_username == admin_username.strip() and not any(ord(c) < 32 for c in admin_username),
                    "A valid new administrator username is required")
            expected = package / "project-secrets" / str(report["tenant_id"]) / report["environment"] / "bootstrap-admin-password"
            require(admin_password_file is not None and admin_password_file.is_file() and
                    not admin_password_file.is_symlink() and
                    admin_password_file.absolute() == expected.absolute() and
                    admin_password_file.resolve() == expected.absolute(),
                    "Administrator password file must be bootstrap-admin-password in this instance's company/environment secret directory")
            require(os.name == "nt" or admin_password_file.stat().st_mode & 0o077 == 0,
                    "Administrator password file must be readable only by its owner (chmod 600)")
            initialize_args += ["--admin-username", admin_username,
                                "--admin-password-file", str(admin_password_file.absolute())]
    else:
        require(document is None and tenant_name is None and admin_user_id is None and
                admin_username is None and admin_password_file is None,
                "Document, company name and administrator options apply only to initialize-config")
    if action == "check":
        print(json.dumps(report, ensure_ascii=False, indent=2))
        return 0
    require(sys.platform.startswith("linux"), "Deployment operations run on the selected Linux host; use check locally")
    require(str(package.absolute()) == report["deploy_root"] and
            str(package.resolve()) == report["deploy_root"],
            "Run against the exact deploy_root from the manifest; copied or redirected paths are refused")
    if action == "sync-config-digest":
        require(report["config_status"] != "pending_config", "No configuration bundle is available to synchronize")
    elif action not in ("status", "doctor", "initialize-config"):
        require(report["config_status"] == "pending_preflight", "Export and install the FND-002 configuration first")
    require(not os.environ.get("DOCKER_HOST") and os.environ.get("DOCKER_CONTEXT", "default") in ("", "default"),
            "Remote Docker settings are present; run on the selected host using its local engine")
    # Do not let inherited Compose/project/path variables redirect this operation.
    child_env = {key: value for key, value in os.environ.items()
                 if key not in env and not key.startswith(("RHD_", "COMPOSE_", "PG_", "DB_"))}
    child_env["DOCKER_CONTEXT"] = "default"
    command = ["bash", str(ROOT / "deploy/1panel/rhdctl"), "--env-file", str(package / ".env"), action, *initialize_args]
    if action == "preflight":
        command.append("--strict")
    print(f"{action}: {instance} ({report['environment']}, host={host})", flush=True)
    return subprocess.run(command, cwd=ROOT, env=child_env, check=False).returncode


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--package", required=True, type=Path)
    parser.add_argument("--instance", required=True)
    parser.add_argument("--host-id", required=True)
    parser.add_argument("--action", required=True, choices=ACTIONS)
    parser.add_argument("--document", type=Path, help="Raw FND-002 document inside the package, for initialization only")
    parser.add_argument("--tenant-name", help="Company display name; existing companies are never renamed")
    parser.add_argument("--admin-user-id", type=int, help="Existing standalone account to bind as company administrator")
    parser.add_argument("--admin-username", help="New company administrator username")
    parser.add_argument("--admin-password-file", type=Path, help="Protected company/environment bootstrap-admin-password file")
    args = parser.parse_args(argv)
    try:
        return operate(args.package, args.instance, args.host_id, args.action, args.document, args.tenant_name,
                       args.admin_user_id, args.admin_username, args.admin_password_file)
    except (InventoryError, OSError, UnicodeError) as exc:
        message = str(exc) if isinstance(exc, InventoryError) else "Cannot access the instance package or command"
        print(f"deploy-environment: {message}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
