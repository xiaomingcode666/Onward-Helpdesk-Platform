"""Exercise instance selection and action routing without invoking Docker."""

import contextlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import MagicMock, patch


SCRIPTS = Path(__file__).resolve().parents[1]
with patch.object(sys, "path", [str(SCRIPTS), *sys.path]):
    import environmentctl
    SPEC = importlib.util.spec_from_file_location("deploy_environment", SCRIPTS / "deploy-environment.py")
    deploy = importlib.util.module_from_spec(SPEC)
    SPEC.loader.exec_module(deploy)


def target(name, port):
    return {"host_id": "local-host", "deploy_root": f"/srv/onward/{name}",
            "public_url": f"https://{name}.example.com", "files_url": f"https://files.{name}.example.com",
            "port_base": port, "config_bundle": ""}


class DeployEnvironmentTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.inventory = self.root / "inventory.json"
        self.inventory.write_text(json.dumps({
            "schema_version": 1, "shared_integration": {"tenant_id": 9001, **target("shared", 18000)},
            "projects": [{"project_id": "alpha", "tenant_id": 101,
                          "staging": target("alpha-staging", 18100),
                          "production": target("alpha-production", 18200)}],
        }))
        environmentctl.render(self.inventory, self.root / "packages")
        self.package = self.root / "packages/onward-alpha-staging"
        self.instance = "onward-alpha-staging"
        self.host = "local-host"

    def change_env(self, **changes):
        path = self.package / ".env"
        values = deploy.read_environment(path)
        values.update(changes)
        path.write_text("\n".join(f"{key}={value}" for key, value in values.items()) + "\n")

    def bundle(self, tenant=101, environment="staging", digest="a" * 64):
        bundle = {"version_id": 7, "digest": digest, "document": {
            "schema_version": 1, "tenant_id": tenant, "environment": environment,
            "projects": [], "intake": {"rules": []}, "secret_refs": [],
        }}
        (self.package / "project-config/current.json").write_text(json.dumps(bundle))
        self.change_env(RHD_PROJECT_CONFIG_DIGEST=digest)

    def test_check_pending_package_has_no_side_effects_or_secret_output(self):
        env = deploy.read_environment(self.package / ".env")
        with patch.object(deploy.subprocess, "run") as run, contextlib.redirect_stdout(io.StringIO()) as output:
            result = deploy.operate(self.package, self.instance, self.host, "check")
        self.assertEqual(result, 0)
        run.assert_not_called()
        report = json.loads(output.getvalue())
        self.assertEqual(report["config_status"], "pending_config")
        self.assertEqual(report["environment"], "staging")
        for key in environmentctl.SECRET_KEYS:
            self.assertNotIn(env[key], output.getvalue())
            self.assertNotIn(key, output.getvalue())

    def test_wrong_instance_and_host_are_rejected_before_any_command(self):
        for instance, host, message in (("onward-alpha-production", self.host, "Selected instance"),
                                        (self.instance, "another-host", "Selected host")):
            with self.subTest(instance=instance, host=host), patch.object(deploy.subprocess, "run") as run:
                with self.assertRaisesRegex(environmentctl.InventoryError, message):
                    deploy.operate(self.package, instance, host, "install")
                run.assert_not_called()

    def test_wrong_env_identity_paths_ports_and_images_are_rejected(self):
        original = (self.package / ".env").read_bytes()
        changes = (
            {"RHD_PROJECT_ENVIRONMENT": "production"}, {"RHD_PROJECT_CONFIG_TENANT_ID": "999"},
            {"RHD_DATA_DIR": "/srv/onward/alpha-production/volumes"},
            {"RHD_PROJECT_NAME": "onward-alpha-production"}, {"RHD_NETWORK_NAME": "shared_network"},
            {"APP_PORT": "18200"}, {"POSTGRES_BIND_IP": "0.0.0.0"},
            {"IMAGE_NAME": "remotehelpdesk"}, {"RHD_RESTORE_DATA_DIR": "/srv/shared/backups"},
            {"RHD_PROJECT_CONFIG_REQUIRED": "0"},
        )
        for change in changes:
            with self.subTest(change=change), patch.object(deploy.subprocess, "run") as run:
                (self.package / ".env").write_bytes(original)
                self.change_env(**change)
                with self.assertRaisesRegex(environmentctl.InventoryError, "differs"):
                    deploy.operate(self.package, self.instance, self.host, "check")
                run.assert_not_called()

    def test_inconsistent_manifest_layout_is_rejected(self):
        path = self.package / "instance.json"
        data = json.loads(path.read_text())
        data["paths"]["RHD_DATA_DIR"] = "/srv/shared/data"
        path.write_text(json.dumps(data))
        with self.assertRaisesRegex(environmentctl.InventoryError, "layout"):
            deploy.inspect_package(self.package, self.instance, self.host)

    def test_bundle_scope_and_pinned_digest_must_match(self):
        with patch.object(deploy.subprocess, "run") as run:
            self.bundle(environment="production")
            with self.assertRaisesRegex(environmentctl.InventoryError, "does not match"):
                deploy.operate(self.package, self.instance, self.host, "check")
            self.bundle(tenant=999)
            with self.assertRaisesRegex(environmentctl.InventoryError, "does not match"):
                deploy.operate(self.package, self.instance, self.host, "check")
            self.bundle()
            self.change_env(RHD_PROJECT_CONFIG_DIGEST="b" * 64)
            with self.assertRaisesRegex(environmentctl.InventoryError, "digest differ"):
                deploy.operate(self.package, self.instance, self.host, "check")
            run.assert_not_called()
        self.bundle()
        report, _ = deploy.inspect_package(self.package, self.instance, self.host)
        self.assertEqual(report["config_status"], "pending_preflight")

    def test_env_substitutions_and_duplicate_assignments_fail_without_leaking_values(self):
        path = self.package / ".env"
        original = path.read_bytes()
        for tail in ("\nOTHER_VALUE=$(touch-private-marker)\n", "\nRHD_PROJECT_NAME=secret-marker-value\n"):
            path.write_bytes(original + tail.encode())
            with patch.object(deploy.subprocess, "run") as run, contextlib.redirect_stderr(io.StringIO()) as errors:
                status = deploy.main(["--package", str(self.package), "--instance", self.instance,
                                      "--host-id", self.host, "--action", "check"])
            self.assertEqual(status, 2)
            self.assertNotIn("private-marker", errors.getvalue())
            self.assertNotIn("secret-marker-value", errors.getvalue())
            run.assert_not_called()

    def test_invalid_utf8_has_safe_error_without_traceback_or_secret_bytes(self):
        (self.package / ".env").write_bytes(b"SECRET=private-marker-\xff\xfe\n")
        with patch.object(deploy.subprocess, "run") as run, contextlib.redirect_stderr(io.StringIO()) as errors:
            status = deploy.main(["--package", str(self.package), "--instance", self.instance,
                                  "--host-id", self.host, "--action", "check"])
        self.assertEqual(status, 2)
        self.assertIn("Cannot access", errors.getvalue())
        self.assertNotIn("private-marker", errors.getvalue())
        self.assertNotIn("Traceback", errors.getvalue())
        run.assert_not_called()

    def test_offline_package_cannot_deploy_from_wrong_directory(self):
        self.bundle()
        with patch.object(deploy.sys, "platform", "linux"), patch.object(deploy.subprocess, "run") as run:
            with self.assertRaisesRegex(environmentctl.InventoryError, "exact deploy_root"):
                deploy.operate(self.package, self.instance, self.host, "install")
            run.assert_not_called()

    def test_correct_action_routes_only_to_selected_environment_file(self):
        self.bundle()
        report, env = deploy.inspect_package(self.package, self.instance, self.host)
        installed = MagicMock(spec=Path)
        installed.absolute.return_value = report["deploy_root"]
        installed.resolve.return_value = report["deploy_root"]
        installed.__truediv__.return_value = Path("selected-private.env")
        inherited = {"PATH": "/usr/bin", "RHD_PROJECT_NAME": "wrong-instance", "COMPOSE_PROJECT_NAME": "wrong",
                     "PG_DOCKER_COMPOSE_PROJECT_NAME": "wrong", "POSTGRES_PASSWORD": "never-pass-inherited",
                     "IMAGE_NAME": "wrong-image", "DB_HOST": "wrong-database"}
        for action in ("preflight", "install", "upgrade", "status", "doctor", "backup", "restore-drill"):
            with self.subTest(action=action), patch.object(deploy, "inspect_package", return_value=(report, env)), \
                    patch.object(deploy.sys, "platform", "linux"), patch.dict(deploy.os.environ, inherited, clear=True), \
                    patch.object(deploy.subprocess, "run", return_value=subprocess.CompletedProcess([], 17)) as run, \
                    contextlib.redirect_stdout(io.StringIO()) as output:
                self.assertEqual(deploy.operate(installed, self.instance, self.host, action), 17)
                command = ["bash", str(deploy.ROOT / "deploy/1panel/rhdctl"), "--env-file", "selected-private.env", action]
                if action == "preflight":
                    command.append("--strict")
                self.assertEqual(run.call_args.args[0], command)
                child_env = run.call_args.kwargs["env"]
                for key in inherited:
                    if key != "PATH":
                        self.assertNotIn(key, child_env)
                self.assertEqual(child_env["PATH"], "/usr/bin")
                self.assertEqual(child_env["DOCKER_CONTEXT"], "default")
                self.assertFalse(run.call_args.kwargs["check"])
                for key in environmentctl.SECRET_KEYS:
                    self.assertNotIn(env[key], output.getvalue())

    def test_remote_docker_settings_cannot_redirect_a_selected_local_instance(self):
        self.bundle()
        report, env = deploy.inspect_package(self.package, self.instance, self.host)
        installed = MagicMock(spec=Path)
        installed.absolute.return_value = installed.resolve.return_value = report["deploy_root"]
        for inherited in ({"DOCKER_HOST": "tcp://unrelated-host:2375"}, {"DOCKER_CONTEXT": "another-customer"}):
            with self.subTest(inherited=inherited), patch.object(deploy, "inspect_package", return_value=(report, env)), \
                    patch.object(deploy.sys, "platform", "linux"), patch.dict(deploy.os.environ, inherited, clear=True), \
                    patch.object(deploy.subprocess, "run") as run:
                with self.assertRaisesRegex(environmentctl.InventoryError, "Remote Docker"):
                    deploy.operate(installed, self.instance, self.host, "upgrade")
                run.assert_not_called()

    def test_pending_configuration_cannot_install_even_at_correct_location(self):
        report, env = deploy.inspect_package(self.package, self.instance, self.host)
        installed = MagicMock(spec=Path)
        installed.absolute.return_value = installed.resolve.return_value = report["deploy_root"]
        with patch.object(deploy, "inspect_package", return_value=(report, env)), \
                patch.object(deploy.sys, "platform", "linux"), patch.object(deploy.subprocess, "run") as run:
            with self.assertRaisesRegex(environmentctl.InventoryError, "configuration first"):
                deploy.operate(installed, self.instance, self.host, "install")
            run.assert_not_called()

    def test_initialize_requires_document_inside_selected_package_and_valid_company_name(self):
        document = self.package / "project-config/initial.json"
        document.write_text('{"schema_version":1,"tenant_id":101,"environment":"staging"}')
        outside = self.root / "another-company.json"
        outside.write_text("{}")
        nested = document.parent / "nested/initial.json"
        nested.parent.mkdir()
        nested.write_text("{}")
        symlink_document = MagicMock(spec=Path)
        symlink_document.is_file.return_value = True
        symlink_document.is_symlink.return_value = True
        cases = (
            (None, "客户公司", "Initial document"),
            (outside, "客户公司", "Initial document"),
            (nested, "客户公司", "Initial document"),
            (document.parent / "missing.json", "客户公司", "Initial document"),
            (symlink_document, "客户公司", "Initial document"),
            (document, None, "company display name"),
            (document, "   ", "company display name"),
            (document, "客户\n公司", "company display name"),
            (document, "公" * 201, "company display name"),
        )
        for supplied, name, error in cases:
            with self.subTest(name=name, error=error), patch.object(deploy.subprocess, "run") as run:
                with self.assertRaisesRegex(environmentctl.InventoryError, error):
                    deploy.operate(self.package, self.instance, self.host, "initialize-config", supplied, name)
                run.assert_not_called()

    def test_initialize_existing_bundle_routes_to_backend_identity_and_content_checks(self):
        self.bundle()
        document = self.package / "project-config/initial.json"
        document.write_text("{}")
        report, env = deploy.inspect_package(self.package, self.instance, self.host)
        report["deploy_root"] = str(self.package.absolute())
        with patch.object(deploy, "inspect_package", return_value=(report, env)), \
                patch.object(deploy.sys, "platform", "linux"), patch.dict(deploy.os.environ, {}, clear=True), \
                patch.object(deploy.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as run:
            self.assertEqual(deploy.operate(self.package, self.instance, self.host, "initialize-config", document, "客户公司", admin_user_id=7), 0)
        self.assertEqual(run.call_args.args[0][-2:], ["--admin-user-id", "7"])

    def test_initialize_admin_options_are_scoped_and_not_password_arguments(self):
        report, env = deploy.inspect_package(self.package, self.instance, self.host)
        report["deploy_root"] = str(self.package.absolute())
        document = self.package / "project-config/initial.json"
        document.write_text("{}")
        password_file = self.package / "project-secrets/101/staging/bootstrap-admin-password"
        password_file.parent.mkdir(parents=True, exist_ok=True)
        password_file.write_text("Fixture-password-never-in-arguments")
        password_file.chmod(0o600)
        with patch.object(deploy, "inspect_package", return_value=(report, env)), \
                patch.object(deploy.sys, "platform", "linux"), patch.dict(deploy.os.environ, {}, clear=True), \
                patch.object(deploy.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as run, \
                contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertEqual(deploy.operate(self.package, self.instance, self.host, "initialize-config", document, "客户公司",
                                          admin_username="fixture.admin", admin_password_file=password_file), 0)
            self.assertEqual(run.call_args.args[0][-4:], ["--admin-username", "fixture.admin", "--admin-password-file", str(password_file.absolute())])
            self.assertNotIn("Fixture-password-never-in-arguments", str(run.call_args))
            self.assertNotIn("Fixture-password-never-in-arguments", output.getvalue())
            for options in ({"admin_user_id": 7, "admin_username": "admin", "admin_password_file": password_file},
                            {"admin_user_id": 0}, {"admin_username": "admin"},
                            {"admin_username": "admin", "admin_password_file": document},
                            {"admin_username": "admin", "admin_password_file": password_file.parent / "missing"}):
                with self.assertRaises(environmentctl.InventoryError):
                    deploy.operate(self.package, self.instance, self.host, "initialize-config", document, "客户公司", **options)

    def test_document_options_cannot_be_used_for_normal_operations(self):
        document = self.package / "project-config/initial.json"
        document.write_text("{}")
        for action in ("check", "install", "status", "upgrade"):
            with self.subTest(action=action), patch.object(deploy.subprocess, "run") as run:
                with self.assertRaisesRegex(environmentctl.InventoryError, "only to initialize-config"):
                    deploy.operate(self.package, self.instance, self.host, action, document, "客户公司")
                run.assert_not_called()

    def test_initialize_routes_document_and_company_name_as_exact_arguments(self):
        report, env = deploy.inspect_package(self.package, self.instance, self.host)
        # Simulate the generated package having been transferred to its Linux root.
        report["deploy_root"] = str(self.package.absolute())
        document = self.package / "project-config/initial.json"
        document.write_text('{"schema_version":1,"tenant_id":101,"environment":"staging"}')
        company_name = " 客户公司 'A' $(not-executed) "
        with patch.object(deploy, "inspect_package", return_value=(report, env)), \
                patch.object(deploy.sys, "platform", "linux"), patch.dict(deploy.os.environ, {"PATH": "/usr/bin"}, clear=True), \
                patch.object(deploy.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as run, \
                contextlib.redirect_stdout(io.StringIO()) as output:
            result = deploy.operate(self.package, self.instance, self.host, "initialize-config", document, company_name)
        self.assertEqual(result, 0)
        self.assertEqual(run.call_args.args[0], [
            "bash", str(deploy.ROOT / "deploy/1panel/rhdctl"), "--env-file", str(self.package / ".env"),
            "initialize-config", "--document", str(document.resolve()), "--tenant-name", company_name.strip(),
        ])
        self.assertNotIn("shell", run.call_args.kwargs)
        self.assertEqual(run.call_args.kwargs["env"]["DOCKER_CONTEXT"], "default")
        for key in environmentctl.SECRET_KEYS:
            self.assertNotIn(env[key], output.getvalue())

    def test_digest_sync_accepts_only_digest_difference_and_routes_to_validator(self):
        self.bundle()
        self.change_env(RHD_PROJECT_CONFIG_DIGEST="b" * 64)
        with self.assertRaisesRegex(environmentctl.InventoryError, "digest differ"):
            deploy.inspect_package(self.package, self.instance, self.host)
        report, env = deploy.inspect_package(self.package, self.instance, self.host, allow_digest_sync=True)
        self.assertEqual(report["config_status"], "pending_digest_sync")
        report["deploy_root"] = str(self.package.absolute())
        with patch.object(deploy, "inspect_package", return_value=(report, env)) as inspect, \
                patch.object(deploy.sys, "platform", "linux"), patch.dict(deploy.os.environ, {}, clear=True), \
                patch.object(deploy.subprocess, "run", return_value=subprocess.CompletedProcess([], 0)) as run, \
                contextlib.redirect_stdout(io.StringIO()) as output:
            self.assertEqual(deploy.operate(self.package, self.instance, self.host, "sync-config-digest"), 0)
        inspect.assert_called_once_with(self.package, self.instance, self.host, allow_digest_sync=True)
        self.assertEqual(run.call_args.args[0], ["bash", str(deploy.ROOT / "deploy/1panel/rhdctl"),
                         "--env-file", str(self.package / ".env"), "sync-config-digest"])
        self.assertEqual(deploy.read_environment(self.package / ".env")["RHD_PROJECT_CONFIG_DIGEST"], "b" * 64,
                         "Python must not rewrite the digest before the authoritative Go validation")
        for key in environmentctl.SECRET_KEYS:
            self.assertNotIn(env[key], output.getvalue())
        self.bundle(environment="production")
        with self.assertRaisesRegex(environmentctl.InventoryError, "does not match"):
            deploy.inspect_package(self.package, self.instance, self.host, allow_digest_sync=True)
        self.bundle()
        self.change_env(RHD_PROJECT_CONFIG_DIR="/srv/another-project/project-config")
        with self.assertRaisesRegex(environmentctl.InventoryError, "differs"):
            deploy.inspect_package(self.package, self.instance, self.host, allow_digest_sync=True)

    def test_digest_sync_cannot_fabricate_missing_configuration(self):
        report, env = deploy.inspect_package(self.package, self.instance, self.host)
        report["deploy_root"] = str(self.package.absolute())
        with patch.object(deploy, "inspect_package", return_value=(report, env)), \
                patch.object(deploy.sys, "platform", "linux"), patch.object(deploy.subprocess, "run") as run:
            with self.assertRaisesRegex(environmentctl.InventoryError, "No configuration bundle"):
                deploy.operate(self.package, self.instance, self.host, "sync-config-digest")
            run.assert_not_called()


if __name__ == "__main__":
    unittest.main()
