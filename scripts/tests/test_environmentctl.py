"""Offline contract checks for the FND-003 deployment inventory and packages."""

import contextlib
import importlib.util
import io
import json
import os
from pathlib import Path
import stat
import tempfile
import unittest


SCRIPT = Path(__file__).resolve().parents[1] / "environmentctl.py"
SPEC = importlib.util.spec_from_file_location("environmentctl", SCRIPT)
environmentctl = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(environmentctl)


def target(name, port, host="test-host"):
    return {
        "host_id": host,
        "deploy_root": f"/srv/onward/{name}",
        "public_url": f"https://{name}.example.com",
        "files_url": f"https://files.{name}.example.com",
        "port_base": port,
        "config_bundle": "",
    }


def inventory():
    return {
        "schema_version": 1,
        "shared_integration": {"tenant_id": 9001, **target("integration", 18000)},
        "projects": [
            {"project_id": "alpha", "tenant_id": 101,
             "staging": target("alpha-staging", 18100), "production": target("alpha-production", 18200)},
            {"project_id": "beta", "tenant_id": 102,
             "staging": target("beta-staging", 18300), "production": target("beta-production", 18400)},
        ],
    }


def read_env(path):
    return dict(line.split("=", 1) for line in path.read_text().splitlines() if line and not line.startswith("#"))


class EnvironmentInventoryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / "inventory.json"
        self.output = self.root / "packages"

    def save(self, value):
        self.source.write_text(json.dumps(value), encoding="utf-8")
        return self.source

    def assert_invalid(self, value, message):
        with self.assertRaisesRegex(environmentctl.InventoryError, message):
            environmentctl.load_targets(self.save(value))

    def test_shared_and_two_projects_generate_five_isolated_instances(self):
        self.save(inventory())
        report = environmentctl.render(self.source, self.output)
        self.assertEqual(len(report["instances"]), 5)
        self.assertFalse(report["deployable"])
        self.assertEqual(report["instances"][0]["instance_id"], "onward-shared-integration")
        all_ports, all_roots, credentials, image_names = set(), set(), set(), set()
        for instance in report["instances"]:
            package = self.output / instance["instance_id"]
            env = read_env(package / ".env")
            self.assertEqual(env["IMAGE_NAME"], instance["instance_id"])
            self.assertNotIn(env["IMAGE_NAME"], image_names)
            image_names.add(env["IMAGE_NAME"])
            self.assertEqual(env["RHD_PROJECT_NAME"], instance["instance_id"])
            self.assertEqual(env["RHD_NETWORK_NAME"], instance["instance_id"] + "_1panel")
            self.assertEqual(env["RHD_PROJECT_ENVIRONMENT"], instance["environment"])
            self.assertEqual(env["RHD_PROJECT_CONFIG_TENANT_ID"], str(instance["tenant_id"]))
            self.assertEqual(env["RHD_PROJECT_CONFIG_DIGEST"], "")
            self.assertEqual(env["RHD_PROJECT_CONFIG_REQUIRED"], "1")
            for key in environmentctl.PORT_KEYS:
                self.assertNotIn(env[key], all_ports)
                all_ports.add(env[key])
            self.assertNotIn(instance["deploy_root"], all_roots)
            all_roots.add(instance["deploy_root"])
            for key in environmentctl.SECRET_KEYS:
                self.assertRegex(env[key], r"^[a-f0-9]{64}$")
                self.assertNotIn(env[key], credentials)
                credentials.add(env[key])
                self.assertNotIn(env[key], json.dumps(report))
            for key, value in instance["paths"].items():
                self.assertEqual(env[key], value)
                self.assertTrue(value.startswith(instance["deploy_root"] + "/"))
            metadata = json.loads((package / "instance.json").read_text())
            self.assertEqual(metadata["ports"], instance["ports"])
            self.assertEqual(metadata["paths"], instance["paths"])
            self.assertNotIn("config_bundle", metadata)
            self.assertTrue((package / "remotehelpdesk.yaml").is_file())
            self.assertTrue((package / "alertmanager.yml").is_file())
            self.assertFalse((package / "project-config/current.json").exists())
            secret_dir = package / "project-secrets" / str(instance["tenant_id"]) / instance["environment"]
            self.assertTrue(secret_dir.is_dir())
            self.assertEqual(list(secret_dir.iterdir()), [])
            if os.name != "nt":
                self.assertEqual(stat.S_IMODE((package / ".env").stat().st_mode), 0o600)
                self.assertEqual(stat.S_IMODE((package / "remotehelpdesk.yaml").stat().st_mode), 0o644)
                self.assertEqual(stat.S_IMODE((package / "alertmanager.yml").stat().st_mode), 0o644)
                self.assertEqual(stat.S_IMODE((package / "project-config").stat().st_mode), 0o755)
                self.assertEqual(stat.S_IMODE(secret_dir.stat().st_mode), 0o700)

    def test_render_refuses_existing_instance_without_resetting_credentials(self):
        self.save(inventory())
        environmentctl.render(self.source, self.output)
        env_path = self.output / "onward-shared-integration/.env"
        original = env_path.read_bytes()
        with self.assertRaisesRegex(environmentctl.InventoryError, "already exists"):
            environmentctl.render(self.source, self.output)
        self.assertEqual(env_path.read_bytes(), original)

    def test_last_target_failure_writes_nothing(self):
        data = inventory()
        data["projects"][-1]["production"]["port_base"] = 18001
        self.save(data)
        with self.assertRaisesRegex(environmentctl.InventoryError, "Ports overlap"):
            environmentctl.render(self.source, self.output)
        self.assertFalse(self.output.exists())
        self.save(inventory())
        self.output.mkdir()
        (self.output / "onward-beta-production").mkdir()
        with self.assertRaisesRegex(environmentctl.InventoryError, "already exists"):
            environmentctl.render(self.source, self.output)
        self.assertFalse((self.output / "onward-shared-integration").exists())

    def test_conflicting_ports_and_directories_on_same_host_rejected(self):
        for root in ("/srv/onward/alpha-staging", "/srv/onward/alpha-staging/nested", "/srv/onward"):
            data = inventory()
            data["projects"][0]["production"]["deploy_root"] = root
            self.assert_invalid(data, "directories overlap")
        data = inventory()
        data["projects"][0]["production"]["port_base"] = 18108
        self.assert_invalid(data, "Ports overlap")

    def test_distinct_hosts_can_reuse_private_ports_and_paths(self):
        data = inventory()
        data["projects"][0]["production"].update(
            host_id="another-host", port_base=18100, deploy_root="/srv/onward/alpha-staging")
        self.assertEqual(len(environmentctl.load_targets(self.save(data))), 5)

    def test_hostname_conflicts_are_rejected_even_on_distinct_hosts(self):
        data = inventory()
        data["projects"][0]["production"].update(
            host_id="another-host", files_url="https://ALPHA-STAGING.example.com")
        self.assert_invalid(data, "hostnames overlap")
        data = inventory()
        data["projects"][0]["production"]["files_url"] = data["projects"][0]["production"]["public_url"]
        self.assert_invalid(data, "distinct hostnames")

    def test_invalid_identity_and_missing_pairs_rejected(self):
        mutations = (
            (lambda d: d.update(schema_version=True), "schema_version"),
            (lambda d: d.update(shared_integration=[]), "expected an object"),
            (lambda d: d.update(shared_integrations=[]), "unsupported fields"),
            (lambda d: d["projects"][0].pop("staging"), "unsupported fields"),
            (lambda d: d["projects"][1].update(project_id="alpha"), "duplicated"),
            (lambda d: d["projects"][0].update(project_id="daypop-shared"), "reserved"),
            (lambda d: d["projects"][0].update(project_id="shared"), "reserved"),
            (lambda d: d["projects"][0].update(project_id="1alpha"), "project_id"),
            (lambda d: d["projects"][0].update(project_id="alpha-"), "project_id"),
            (lambda d: d["projects"][0].update(project_id="alpha--beta"), "project_id"),
            (lambda d: d["projects"][0].update(project_id="a" * 49), "project_id"),
            (lambda d: d["projects"][0].update(tenant_id=True), "positive integer"),
            (lambda d: d["projects"][1].update(tenant_id=101), "distinct company"),
            (lambda d: d["projects"][0]["staging"].update(tenant_id=999), "unsupported fields"),
            (lambda d: d["shared_integration"].update(port_base=65530), "positive integer"),
            (lambda d: d["shared_integration"].update(port_base=80), "unprivileged"),
        )
        for change, message in mutations:
            with self.subTest(message=message):
                data = inventory()
                change(data)
                self.assert_invalid(data, message)

    def test_unsafe_paths_and_shell_values_rejected(self):
        for path in ("/", "/home/user", "/var/lib", "/etc/onward", "/srv/../etc", "/srv/a/", "/srv//a",
                     "/srv/a\nTOKEN=evil", "/srv/$(id)", "/srv/`id`", "C:/srv/test", "/srv/a;id"):
            data = inventory()
            data["shared_integration"]["deploy_root"] = path
            with self.subTest(path=path):
                self.assert_invalid(data, "deploy_root")
        for url in ("http://app.example.com", "https://user:pass@app.example.com", "https://app.example.com/path",
                    "https://app.example.com\nTOKEN=evil", "https://app.example.com?x=1", "https://$(id).example.com"):
            data = inventory()
            data["shared_integration"]["public_url"] = url
            with self.subTest(url=url):
                self.assert_invalid(data, "public_url")
        data = inventory()
        data["shared_integration"]["host_id"] = "host;touch"
        self.assert_invalid(data, "host_id")

    def test_matching_export_is_copied_exactly_and_mismatch_is_not_rewritten(self):
        data = inventory()
        bundle = {"version_id": 12, "digest": "a" * 64, "document": {
            "schema_version": 1, "tenant_id": 9001, "environment": "integration",
            "projects": [], "intake": {"rules": []}, "secret_refs": [],
        }}
        source = self.root / "integration.json"
        raw = json.dumps(bundle, indent=4).encode()
        source.write_bytes(raw)
        data["shared_integration"]["config_bundle"] = "integration.json"
        environmentctl.render(self.save(data), self.output)
        copied = self.output / "onward-shared-integration/project-config/current.json"
        self.assertEqual(copied.read_bytes(), raw)
        if os.name != "nt":
            self.assertEqual(stat.S_IMODE(copied.stat().st_mode), 0o644)
        env = read_env(self.output / "onward-shared-integration/.env")
        self.assertEqual(env["RHD_PROJECT_CONFIG_DIGEST"], "a" * 64)
        bundle["document"]["environment"] = "production"
        source.write_text(json.dumps(bundle))
        self.assert_invalid(data, "does not match")
        bundle["document"].update(environment="integration", tenant_id=101)
        source.write_text(json.dumps(bundle))
        self.assert_invalid(data, "does not match")

    def test_duplicate_json_and_secret_fields_are_rejected(self):
        self.source.write_text('{"schema_version":1,"schema_version":1}')
        with self.assertRaisesRegex(environmentctl.InventoryError, "duplicate"):
            environmentctl.load_targets(self.source)
        data = inventory()
        data["projects"][0]["production"]["POSTGRES_PASSWORD"] = "must-not-accept"
        self.assert_invalid(data, "unsupported fields")

    def test_select_stdout_is_one_json_object_and_unknown_target_fails(self):
        self.save(inventory())
        stdout, stderr = io.StringIO(), io.StringIO()
        with contextlib.redirect_stdout(stdout), contextlib.redirect_stderr(stderr):
            result = environmentctl.main(["select", "--inventory", str(self.source),
                                          "--instance", "onward-beta-production"])
        self.assertEqual(result, 0)
        self.assertEqual(stderr.getvalue(), "")
        metadata = json.loads(stdout.getvalue())
        self.assertEqual(metadata["tenant_id"], 102)
        self.assertEqual(metadata["environment"], "production")
        self.assertEqual(metadata["deploy_root"], "/srv/onward/beta-production")
        self.assertNotIn("config_bundle", metadata)
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            self.assertEqual(environmentctl.main(["select", "--inventory", str(self.source),
                                                 "--instance", "missing"]), 2)


if __name__ == "__main__":
    unittest.main()
