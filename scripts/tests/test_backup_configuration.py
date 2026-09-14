"""Failure-boundary checks for restoring a configuration and its env digest."""

import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch


SCRIPT = Path(__file__).resolve().parents[1] / "lib/deployment-backup-config.py"
SPEC = importlib.util.spec_from_file_location("backup_config", SCRIPT)
helper = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(helper)


class BackupConfigurationTests(unittest.TestCase):
    def test_failed_env_publish_keeps_recoverable_configuration_and_cleans_temporary_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory).resolve()
            config = root / "project-config/current.json"
            config.parent.mkdir()
            config.write_text("old deployment package", encoding="utf-8")
            env = root / ".env"
            old_digest, digest = "a" * 64, "b" * 64
            original = f"RHD_PROJECT_ENVIRONMENT=staging\nRHD_PROJECT_CONFIG_TENANT_ID=11\nRHD_PROJECT_CONFIG_DIGEST={old_digest}\nPRIVATE_VALUE=retain-me\n"
            env.write_text(original, encoding="utf-8")
            bundle = root / "old-backup.config.json"
            bundle.write_text(json.dumps({"version_id": 7, "digest": digest, "document": {
                "schema_version": 1, "tenant_id": 11, "environment": "staging",
            }}), encoding="utf-8")
            variables = {"RHD_PROJECT_ENVIRONMENT": "staging", "RHD_PROJECT_CONFIG_TENANT_ID": "11",
                         "RHD_PROJECT_CONFIG_FILE": str(config), "RHD_ENV_FILE": str(env)}
            replace = os.replace

            def fail_env(source, target):
                if target == env:
                    raise PermissionError("synthetic publication failure")
                return replace(source, target)

            with patch.dict(os.environ, variables, clear=True), \
                    patch.object(sys, "argv", [str(SCRIPT), "activate", str(bundle)]), \
                    patch.object(helper.os, "replace", side_effect=fail_env), \
                    contextlib.redirect_stderr(io.StringIO()) as error:
                self.assertEqual(helper.main(), 1)
            self.assertIn("must stay stopped", error.getvalue())
            self.assertEqual(config.read_bytes(), bundle.read_bytes())
            self.assertEqual(env.read_text(encoding="utf-8"), original)
            self.assertEqual(list(root.glob("**/.restore-config-*")), [])

            # A compact JSON package remains readable and an explicit retry can
            # repair the digest without losing the unrelated private env value.
            with patch.dict(os.environ, variables, clear=True), \
                    patch.object(sys, "argv", [str(SCRIPT), "activate", str(bundle)]):
                self.assertEqual(helper.main(), 0)
            self.assertIn("RHD_PROJECT_CONFIG_DIGEST=" + digest, env.read_text(encoding="utf-8"))
            self.assertIn("PRIVATE_VALUE=retain-me", env.read_text(encoding="utf-8"))


if __name__ == "__main__":
    unittest.main()
