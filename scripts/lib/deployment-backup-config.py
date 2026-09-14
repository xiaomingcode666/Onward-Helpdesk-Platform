#!/usr/bin/env python3
"""Small JSON/file helper for managed backups; never prints configuration data."""

from __future__ import annotations

import argparse
import os
from pathlib import Path
import re
import sys
import tempfile

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from environmentctl import InventoryError, read_bundle, require


def target_files() -> tuple[Path, Path]:
    config = Path(os.environ.get("RHD_PROJECT_CONFIG_FILE", ""))
    env = Path(os.environ.get("RHD_ENV_FILE", ""))
    require(config.is_absolute() and config.name == "current.json" and
            env.is_absolute() and env.name == ".env" and
            config.parent.name == "project-config" and config.parent.parent == env.parent,
            "Managed restore requires the instance current.json and .env paths")
    for path in (config, env, config.parent, env.parent):
        require(not path.is_symlink() and path.absolute() == path.resolve(),
                "Restore configuration paths must not use symbolic links")
    require(config.is_file() and env.is_file(), "Current configuration and env file must exist before restore")
    text = env.read_text(encoding="utf-8-sig")
    keys = ["RHD_PROJECT_ENVIRONMENT", "RHD_PROJECT_CONFIG_TENANT_ID"]
    if os.environ.get("RHD_INSTANCE_ID") or os.environ.get("RHD_DEPLOYMENT_PROJECT_ID"):
        keys += ["RHD_INSTANCE_ID", "RHD_DEPLOYMENT_PROJECT_ID"]
    expected = {key: os.environ.get(key, "") for key in keys}
    seen = {}
    for line in text.splitlines():
        key, separator, value = line.partition("=")
        if key in expected or key == "RHD_PROJECT_CONFIG_DIGEST":
            require(separator and key not in seen, "Restore env identity is missing or ambiguous")
            seen[key] = value.strip().strip("\"'")
    require(all(seen.get(key) == value and value for key, value in expected.items()),
            "Restore env file does not match the selected instance")
    require("RHD_PROJECT_CONFIG_DIGEST" in seen, "Restore env file has no pinned digest")
    return config, env


def staged_file(path: Path, data: bytes, mode: int) -> Path:
    descriptor, name = tempfile.mkstemp(prefix=".restore-config-", dir=path.parent)
    result = Path(name)
    try:
        with os.fdopen(descriptor, "wb") as output:
            output.write(data)
            output.flush()
            os.fsync(output.fileno())
        os.chmod(result, mode)
        return result
    except BaseException:
        result.unlink(missing_ok=True)
        raise


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("summary", "check-target", "activate"))
    parser.add_argument("bundle", type=Path)
    args = parser.parse_args()
    try:
        tenant = int(os.environ.get("RHD_PROJECT_CONFIG_TENANT_ID", "0"))
        bundle, raw = read_bundle(args.bundle, tenant, os.environ.get("RHD_PROJECT_ENVIRONMENT", ""))
        require(args.bundle.is_file() and not args.bundle.is_symlink(), "Backup configuration must be a regular file")
        if args.action == "summary":
            print(f"{bundle['version_id']}|{bundle['digest']}")
            return 0
        config, env = target_files()
        env_text = env.read_text(encoding="utf-8-sig")
        replacement = re.sub(r"(?m)^RHD_PROJECT_CONFIG_DIGEST=.*$",
                             "RHD_PROJECT_CONFIG_DIGEST=" + bundle["digest"], env_text)
        config_tmp = env_tmp = None
        try:
            # Probe and prepare both writes before replacing either existing file.
            config_tmp = staged_file(config, raw, 0o644)
            env_tmp = staged_file(env, replacement.encode("utf-8"), 0o600)
            if args.action == "activate":
                os.replace(config_tmp, config)
                config_tmp = None
                os.replace(env_tmp, env)
                env_tmp = None
        finally:
            for path in (config_tmp, env_tmp):
                if path is not None:
                    path.unlink(missing_ok=True)
        return 0
    except (InventoryError, OSError, UnicodeError, ValueError):
        print("Managed backup configuration is invalid, unreadable, or cannot be safely installed; application must stay stopped.", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
