#!/usr/bin/env python3
"""Validate a FND-003 inventory and prepare isolated Linux deployment packages.

Uses only the Python standard library. Never reads an operator's .env, deploys,
or replaces an existing package. FND-002 preflight remains the authority for
configuration schema, policy, digest, and secret-reference verification.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path, PurePosixPath
import re
import secrets
import sys
from urllib.parse import urlsplit

# The test suite loads this file by path, while deployment runs it as a script.
# Make the sibling provenance module available in both modes.
_SCRIPT_DIR = str(Path(__file__).resolve().parent)
if _SCRIPT_DIR not in sys.path:
    sys.path.insert(0, _SCRIPT_DIR)
from data_provenance import default_synthetic


ROOT = Path(__file__).resolve().parents[1]
IDENTIFIER = re.compile(r"[a-z][a-z0-9]*(?:-[a-z0-9]+)*\Z")
PORT_KEYS = (
    "APP_PORT", "POSTGRES_PORT", "REDIS_PORT", "QDRANT_HTTP_PORT",
    "QDRANT_GRPC_PORT", "MINIO_PORT", "MINIO_CONSOLE_PORT",
    "PROMETHEUS_PORT", "ALERTMANAGER_PORT",
)
SECRET_KEYS = (
    "POSTGRES_PASSWORD", "REDIS_PASSWORD", "MINIO_ROOT_PASSWORD",
    "CUSTOMER_SESSION_SECRET", "ENCRYPTION_KEY", "MCP_SERVER_TOKEN",
    "RHD_BOOTSTRAP_ADMIN_PASSWORD",
)
TARGET_KEYS = {
    "host_id", "deploy_root", "public_url", "files_url", "port_base", "config_bundle",
}


class InventoryError(ValueError):
    """A safe diagnostic that does not include supplied secrets or file contents."""


def require(condition: bool, message: str) -> None:
    if not condition:
        raise InventoryError(message)


def object_keys(value: object, keys: set[str], label: str) -> dict:
    require(isinstance(value, dict), f"{label}: expected an object")
    require(set(value) == keys, f"{label}: missing or unsupported fields")
    return value


def positive_int(value: object, label: str, maximum: int = 2**63 - 1) -> int:
    require(type(value) is int and 0 < value <= maximum, f"{label}: expected a positive integer")
    return value


def identifier(value: object, label: str) -> str:
    require(isinstance(value, str) and len(value) <= 48 and IDENTIFIER.fullmatch(value) is not None,
            f"{label}: start with a lowercase letter; use up to 48 letters, digits and single separating hyphens")
    return value


def deployment_root(value: object) -> str:
    require(isinstance(value, str) and re.fullmatch(r"/[a-zA-Z0-9_./-]+", value) is not None,
            "deploy_root: use a normalized absolute Linux path without shell characters")
    path = PurePosixPath(value)
    allowed = ("/srv/", "/opt/", "/var/lib/", "/data/", "/mnt/")
    require(str(path) == value and ".." not in path.parts and
            any(value.startswith(prefix) and value != prefix.rstrip("/") for prefix in allowed),
            "deploy_root: use an instance directory under /srv, /opt, /var/lib, /data or /mnt")
    require(len(path.parts) >= 3, "deploy_root: a dedicated instance directory is required")
    return value


def public_origin(value: object, label: str) -> tuple[str, str]:
    require(isinstance(value, str) and re.fullmatch(r"https://[a-zA-Z0-9.:-]+/?", value) is not None,
            f"{label}: use an HTTPS origin without credentials, paths or shell characters")
    try:
        parsed = urlsplit(value)
        port = parsed.port
    except ValueError as exc:
        raise InventoryError(f"{label}: invalid URL") from exc
    require(bool(parsed.hostname) and parsed.username is None and parsed.password is None and
            parsed.hostname not in ("localhost", "127.0.0.1") and (port is None or 0 < port <= 65535),
            f"{label}: a public hostname is required")
    require(re.fullmatch(r"[a-z0-9](?:[a-z0-9.-]*[a-z0-9])?", parsed.hostname) is not None and
            ".." not in parsed.hostname, f"{label}: invalid hostname")
    return value.rstrip("/"), parsed.hostname.lower().rstrip(".")


def _unique_object(pairs: list[tuple[str, object]]) -> dict:
    result = {}
    for key, value in pairs:
        require(key not in result, "JSON contains a duplicate field")
        result[key] = value
    return result


def read_json(path: Path, limit: int) -> tuple[object, bytes]:
    try:
        with path.open("rb") as source:
            raw = source.read(limit + 1)
        require(len(raw) <= limit, "JSON file exceeds its size limit")
        return json.loads(raw.decode("utf-8-sig"), object_pairs_hook=_unique_object), raw
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise InventoryError("Cannot read a valid UTF-8 JSON file") from exc


def read_bundle(path: Path, tenant_id: int, environment: str) -> tuple[dict, bytes]:
    bundle, raw = read_json(path, 300 * 1024)
    object_keys(bundle, {"version_id", "digest", "document"}, "config_bundle")
    positive_int(bundle["version_id"], "config_bundle.version_id")
    require(isinstance(bundle["digest"], str) and re.fullmatch(r"[a-f0-9]{64}", bundle["digest"]) is not None,
            "config_bundle: missing a valid exported digest")
    doc = bundle["document"]
    require(isinstance(doc, dict) and type(doc.get("tenant_id")) is int and
            doc["tenant_id"] == tenant_id and doc.get("environment") == environment,
            "config_bundle: company or environment does not match the target; export the correct version")
    require(type(doc.get("schema_version")) is int and doc["schema_version"] in (1, 2),
            "config_bundle: unsupported document schema")
    return bundle, raw


def load_targets(inventory_path: Path | str) -> list[dict]:
    """Return fully validated target metadata, never any generated credentials."""
    source = Path(inventory_path).resolve()
    inventory, _ = read_json(source, 1024 * 1024)
    object_keys(inventory, {"schema_version", "shared_integration", "projects"}, "inventory")
    require(type(inventory["schema_version"]) is int and inventory["schema_version"] == 1,
            "inventory: unsupported schema_version")
    shared = object_keys(inventory["shared_integration"], TARGET_KEYS | {"tenant_id"}, "shared_integration")
    require(isinstance(inventory["projects"], list) and 0 < len(inventory["projects"]) <= 500,
            "projects: supply 1-500 projects, each with staging and production")
    targets = []
    projects = set()
    tenants = set()

    def add_target(settings: dict, project: str, tenant_id: int, environment: str) -> None:
        host = identifier(settings["host_id"], "host_id")
        root = deployment_root(settings["deploy_root"])
        public, public_host = public_origin(settings["public_url"], "public_url")
        files, files_host = public_origin(settings["files_url"], "files_url")
        require(public_host != files_host, "Application and file endpoints must use distinct hostnames")
        base = positive_int(settings["port_base"], "port_base", 65535 - len(PORT_KEYS) + 1)
        require(base >= 1024, "port_base: use unprivileged ports (1024 or greater)")
        instance = "onward-shared-integration" if environment == "integration" else f"onward-{project}-{environment}"
        bundle_path = settings["config_bundle"]
        require(isinstance(bundle_path, str) and not any(ord(c) < 32 for c in bundle_path),
                "config_bundle: expected a file path or an empty string")
        digest = ""
        if bundle_path:
            bundle_source = Path(bundle_path)
            if not bundle_source.is_absolute():
                bundle_source = source.parent / bundle_source
            bundle_source = bundle_source.resolve()
            bundle, _ = read_bundle(bundle_source, tenant_id, environment)
            digest = bundle["digest"]
            bundle_path = str(bundle_source)
        paths = {
            "RHD_DATA_DIR": root + "/volumes", "RHD_BACKUP_DIR": root + "/backups",
            "RHD_STATE_DIR": root + "/state", "RHD_METRICS_DIR": root + "/metrics",
            "RHD_CONFIG_FILE": root + "/remotehelpdesk.yaml",
            "RHD_PROJECT_CONFIG_DIR": root + "/project-config",
            "RHD_PROJECT_CONFIG_FILE": root + "/project-config/current.json",
            "RHD_PROJECT_SECRET_DIR": root + "/project-secrets",
            "RHD_TEST_DATA_PROVENANCE_FILE": root + "/data-provenance.json",
            "RHD_ALERTMANAGER_CONFIG_FILE": root + "/alertmanager.yml",
            "RHD_RESTORE_DATA_DIR": root + "/backups",
        }
        targets.append({
            "instance_id": instance, "deployment_project_id": project,
            "environment": environment, "tenant_id": tenant_id, "host_id": host,
            "deploy_root": root, "public_url": public, "files_url": files,
            "port_base": base, "ports": {key: base + i for i, key in enumerate(PORT_KEYS)},
            "paths": paths, "config_bundle": bundle_path, "config_digest": digest,
            "config_status": "pending_preflight" if digest else "pending_config",
        })

    shared_tenant = positive_int(shared["tenant_id"], "shared_integration.tenant_id")
    tenants.add(shared_tenant)
    add_target(shared, "daypop-shared", shared_tenant, "integration")
    for project in inventory["projects"]:
        object_keys(project, {"project_id", "tenant_id", "staging", "production"}, "project")
        project_id = identifier(project["project_id"], "project_id")
        require(project_id not in ("shared", "daypop-shared") and project_id not in projects,
                "project_id is reserved or duplicated")
        projects.add(project_id)
        tenant_id = positive_int(project["tenant_id"], "project.tenant_id")
        require(tenant_id not in tenants, "Each project and shared integration must have an explicit, distinct company mapping")
        tenants.add(tenant_id)
        for environment in ("staging", "production"):
            settings = object_keys(project[environment], TARGET_KEYS, f"project.{environment}")
            add_target(settings, project_id, tenant_id, environment)

    seen_domains = set()
    for index, target in enumerate(targets):
        for key in ("public_url", "files_url"):
            hostname = urlsplit(target[key]).hostname.lower().rstrip(".")
            require(hostname not in seen_domains, "Public hostnames overlap between deployment instances")
            seen_domains.add(hostname)
        for other in targets[:index]:
            if target["host_id"] != other["host_id"]:
                continue
            require(not set(target["ports"].values()) & set(other["ports"].values()),
                    "Ports overlap for instances on the same host")
            one, two = target["deploy_root"], other["deploy_root"]
            require(one != two and not one.startswith(two + "/") and not two.startswith(one + "/"),
                    "Deployment directories overlap for instances on the same host")
    return targets


def public_target(target: dict) -> dict:
    return {key: value for key, value in target.items() if key != "config_bundle"}


def template_environment() -> dict[str, str]:
    # Only the checked-in example is read. Real deployment files contain secrets.
    result = {}
    for line in (ROOT / "deploy/1panel/.env.example").read_text(encoding="utf-8-sig").splitlines():
        if line and not line.lstrip().startswith("#") and "=" in line:
            key, value = line.split("=", 1)
            require(re.fullmatch(r"[A-Z][A-Z0-9_]*", key) is not None, "Invalid environment template key")
            result[key] = value
    return result


def environment_values(target: dict) -> dict[str, str]:
    values = template_environment()
    values.update({
        # Web builds embed public URLs; each instance must keep its own image tag.
        "IMAGE_NAME": target["instance_id"],
        "RHD_INSTANCE_ID": target["instance_id"],
        "RHD_DEPLOYMENT_PROJECT_ID": target["deployment_project_id"],
        "RHD_PROJECT_NAME": target["instance_id"],
        "RHD_NETWORK_NAME": target["instance_id"] + "_1panel",
        "RHD_PROJECT_ENVIRONMENT": target["environment"],
        "RHD_PROJECT_CONFIG_TENANT_ID": str(target["tenant_id"]),
        "RHD_PROJECT_CONFIG_REQUIRED": "1", "RHD_PROJECT_CONFIG_DIGEST": target["config_digest"],
        "RHD_PROJECT_CONFIG_CHECKER": "",  # Existing preflight resolves its own repo/dist default.
        "RHD_PUBLIC_URL": target["public_url"], "MINIO_PUBLIC_ENDPOINT": target["files_url"],
    })
    values.update(target["paths"])
    values.update({key: str(value) for key, value in target["ports"].items()})
    for key in SECRET_KEYS:
        values[key] = secrets.token_hex(32)
    for key in ("APP_BIND_IP", "POSTGRES_BIND_IP", "REDIS_BIND_IP", "QDRANT_BIND_IP",
                "MINIO_BIND_IP", "MINIO_CONSOLE_BIND_IP", "PROMETHEUS_BIND_IP", "ALERTMANAGER_BIND_IP"):
        values[key] = "127.0.0.1"
    for value in values.values():
        require(isinstance(value, str) and not any(character in value for character in "\r\n\x00$`"),
                "Environment values cannot contain substitutions or newlines")
    return values


def write_exclusive(path: Path, content: bytes, mode: int = 0o600) -> None:
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
    with os.fdopen(descriptor, "wb") as output:
        output.write(content)
    os.chmod(path, mode)


def render(inventory_path: Path | str, output_path: Path | str) -> dict:
    targets = load_targets(inventory_path)
    output = Path(output_path).resolve()
    require(not output.exists() or output.is_dir(), "Output must be a directory")
    # Validate every input and every destination before generating any credentials.
    for target in targets:
        require(not os.path.lexists(output / target["instance_id"]),
                "An instance output already exists; refusing to replace files or reset credentials")
    runtime_yaml = (ROOT / "deploy/1panel/remotehelpdesk.yaml").read_bytes()
    alertmanager_yaml = (ROOT / "deploy/1panel/alertmanager.yml").read_bytes()
    prepared = []
    for target in targets:
        bundle_bytes = None
        if target["config_bundle"]:
            bundle, bundle_bytes = read_bundle(Path(target["config_bundle"]), target["tenant_id"], target["environment"])
            require(bundle["digest"] == target["config_digest"], "Configuration export changed during preparation")
        provenance_bytes = None
        if target["environment"] in ("integration", "staging"):
            now = datetime.now(timezone.utc).replace(microsecond=0).isoformat().replace("+00:00", "Z")
            provenance_bytes = (json.dumps(default_synthetic(target["environment"], now), ensure_ascii=False, indent=2) + "\n").encode()
        prepared.append((target, environment_values(target), bundle_bytes, provenance_bytes))
    output.mkdir(parents=True, exist_ok=True)
    for target, env, bundle_bytes, provenance_bytes in prepared:
        destination = output / target["instance_id"]
        destination.mkdir(mode=0o700)  # Exclusive even if another process races the precheck.
        write_exclusive(destination / ".env", ("# Generated FND-003 instance; contains private credentials.\n" +
                        "\n".join(f"{key}={value}" for key, value in env.items()) + "\n").encode())
        write_exclusive(destination / "instance.json", (json.dumps(
            {"schema_version": 1, **public_target(target)}, ensure_ascii=False, indent=2) + "\n").encode())
        # These files hold public settings/placeholders/references, never secrets.
        # The non-root container user must be able to read the mounted settings.
        write_exclusive(destination / "remotehelpdesk.yaml", runtime_yaml, 0o644)
        write_exclusive(destination / "alertmanager.yml", alertmanager_yaml, 0o644)
        if provenance_bytes is not None:
            write_exclusive(destination / "data-provenance.json", provenance_bytes, 0o644)
        (destination / "project-config").mkdir(mode=0o755)
        os.chmod(destination / "project-config", 0o755)
        (destination / "project-secrets" / str(target["tenant_id"]) / target["environment"]).mkdir(mode=0o700, parents=True)
        if bundle_bytes is not None:
            write_exclusive(destination / "project-config/current.json", bundle_bytes, 0o644)
    return {
        "status": "prepared", "deployable": False, "instances": [public_target(target) for target in targets],
        "next_step": "Export missing FND-002 bundles, supply referenced secrets, pin a release and run preflight on each target host.",
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    subcommands = parser.add_subparsers(dest="command", required=True)
    for command in ("validate", "list", "select", "render"):
        subparser = subcommands.add_parser(command)
        subparser.add_argument("--inventory", required=True, type=Path)
        if command == "render":
            subparser.add_argument("--output", required=True, type=Path)
        if command == "select":
            subparser.add_argument("--instance", required=True)
    args = parser.parse_args(argv)
    try:
        if args.command == "render":
            result = render(args.inventory, args.output)
        else:
            targets = load_targets(args.inventory)
            if args.command == "select":
                selected = [target for target in targets if target["instance_id"] == args.instance]
                require(len(selected) == 1, "The requested instance is not in this inventory")
                result = public_target(selected[0])
            elif args.command == "list":
                result = [public_target(target) for target in targets]
            else:
                result = {"valid": True, "instance_count": len(targets),
                          "pending_config": [t["instance_id"] for t in targets if t["config_status"] == "pending_config"]}
        print(json.dumps(result, ensure_ascii=False, indent=2))
        return 0
    except (InventoryError, OSError) as exc:
        message = str(exc) if isinstance(exc, InventoryError) else "Filesystem operation failed; existing files were not replaced"
        print(f"environmentctl: {message}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
