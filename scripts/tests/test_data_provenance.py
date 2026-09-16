"""Acceptance checks for the non-production data provenance gate."""

from __future__ import annotations

import importlib.util
import json
from pathlib import Path
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location("data_provenance", SCRIPTS / "data_provenance.py")
provenance = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(provenance)


class DataProvenanceTests(unittest.TestCase):
    def timestamp(self):
        return "2026-09-16T03:00:00Z"

    def synthetic(self):
        return provenance.default_synthetic("staging", self.timestamp())

    def test_generated_synthetic_attestation_passes(self):
        summary = provenance.validate_document(self.synthetic(), "staging")
        self.assertEqual(summary["classification"], "synthetic")
        self.assertEqual(summary["approval_status"], "not_required")

    def test_approved_sanitized_requires_approval_and_passed_scan(self):
        document = self.synthetic()
        document["classification"] = "approved_sanitized"
        document["approval"] = {"status": "approved", "reference": "APR-2026-001",
                                 "approved_by": "", "approved_at": self.timestamp()}
        with self.assertRaisesRegex(provenance.ProvenanceError, "approved_sanitized"):
            provenance.validate_document(document, "staging")
        document["approval"]["approved_by"] = "data-owner"
        document["verification"]["method"] = "database-and-file-pii-scan"
        document["approval"]["reference"] = "APR-2026-001"
        self.assertEqual(provenance.validate_document(document, "staging")["approval_status"], "approved")
        document["verification"]["pii_scan"] = "failed"
        with self.assertRaisesRegex(provenance.ProvenanceError, "verification"):
            provenance.validate_document(document, "staging")

    def test_missing_manifest_and_production_class_are_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "data-provenance.json"
            with self.assertRaisesRegex(provenance.ProvenanceError, "regular file"):
                provenance.read(path, "integration")
            path.write_text(json.dumps(self.synthetic()), encoding="utf-8")
            with self.assertRaisesRegex(provenance.ProvenanceError, "environment"):
                provenance.read(path, "integration")


if __name__ == "__main__":
    unittest.main()
