#!/usr/bin/env python3
"""Validate the data provenance attestation required by non-production packages.

The deployment package never contains business data.  This small attestation
records whether the target will use synthetic data or an approved, sanitised
dataset.  It deliberately stores references and verification results only; no
customer values are copied into it.
"""

from __future__ import annotations

from datetime import datetime
import json
from pathlib import Path
import re


class ProvenanceError(ValueError):
    """A safe validation error that does not include attestation contents."""


_TOP_LEVEL = {"schema_version", "environment", "classification", "source_ref", "approval", "verification"}
_APPROVAL = {"status", "reference", "approved_by", "approved_at"}
_VERIFICATION = {"status", "method", "checked_by", "checked_at", "pii_scan"}
_ENVIRONMENTS = {"integration", "staging"}
_CLASSIFICATIONS = {"synthetic", "approved_sanitized"}
_UTC_TIMESTAMP = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$")
_REFERENCE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._:/@+ -]{0,199}$")


def _require(condition: bool, message: str) -> None:
    if not condition:
        raise ProvenanceError(message)


def _unique_object(pairs: list[tuple[str, object]]) -> dict:
    result: dict = {}
    for key, value in pairs:
        _require(key not in result, "data provenance contains a duplicate field")
        result[key] = value
    return result


def _text(value: object, label: str, *, required: bool = True) -> str:
    _require(isinstance(value, str), f"data provenance {label} must be text")
    _require(len(value) <= 200 and not any(ord(char) < 32 for char in value),
             f"data provenance {label} is invalid")
    if required:
        _require(bool(value.strip()), f"data provenance {label} is required")
    return value.strip()


def _timestamp(value: object, label: str, *, required: bool = True) -> str:
    result = _text(value, label, required=required)
    if result:
        _require(_UTC_TIMESTAMP.fullmatch(result) is not None,
                 f"data provenance {label} must be a UTC timestamp")
        try:
            datetime.strptime(result, "%Y-%m-%dT%H:%M:%SZ")
        except ValueError as exc:
            raise ProvenanceError(f"data provenance {label} is invalid") from exc
    return result


def validate_document(document: object, environment: str) -> dict:
    """Validate and return a small, safe summary of a provenance document."""
    _require(environment in _ENVIRONMENTS, "data provenance is only required for non-production environments")
    _require(isinstance(document, dict) and set(document) == _TOP_LEVEL,
             "data provenance has missing or unsupported fields")
    _require(document["schema_version"] == 1, "unsupported data provenance schema")
    _require(document["environment"] == environment, "data provenance environment does not match the target")
    classification = _text(document["classification"], "classification")
    _require(classification in _CLASSIFICATIONS, "data provenance classification must be synthetic or approved_sanitized")
    source_ref = _text(document["source_ref"], "source_ref")
    _require(_REFERENCE.fullmatch(source_ref) is not None, "data provenance source_ref is invalid")

    approval = document["approval"]
    _require(isinstance(approval, dict) and set(approval) == _APPROVAL,
             "data provenance approval has missing or unsupported fields")
    approval_status = _text(approval["status"], "approval.status")
    approval_reference = _text(approval["reference"], "approval.reference", required=False)
    approved_by = _text(approval["approved_by"], "approval.approved_by", required=False)
    approved_at = _timestamp(approval["approved_at"], "approval.approved_at", required=False)
    if classification == "synthetic":
        _require(approval_status == "not_required" and not approval_reference and not approved_by and not approved_at,
                 "synthetic data must use not_required approval")
    else:
        _require(approval_status == "approved" and bool(approval_reference) and bool(approved_by) and bool(approved_at),
                 "approved_sanitized data requires an approval reference, approver and time")
        _require(_REFERENCE.fullmatch(approval_reference) is not None,
                 "data provenance approval.reference is invalid")

    verification = document["verification"]
    _require(isinstance(verification, dict) and set(verification) == _VERIFICATION,
             "data provenance verification has missing or unsupported fields")
    verification_status = _text(verification["status"], "verification.status")
    method = _text(verification["method"], "verification.method")
    checked_by = _text(verification["checked_by"], "verification.checked_by")
    checked_at = _timestamp(verification["checked_at"], "verification.checked_at")
    pii_scan = _text(verification["pii_scan"], "verification.pii_scan")
    _require(verification_status == "passed" and pii_scan == "passed",
             "data provenance verification must pass before non-production use")

    return {"environment": environment, "classification": classification,
            "source_ref": source_ref, "approval_status": approval_status,
            "verification_status": verification_status, "checked_at": checked_at,
            "verification_method": method, "checked_by": checked_by}


def read(path: Path, environment: str) -> dict:
    """Read one bounded, non-symlink attestation and validate it."""
    _require(path.is_file() and not path.is_symlink(), "data provenance file must be a regular file")
    _require(path.stat().st_size <= 32 * 1024, "data provenance file is too large")
    try:
        document = json.loads(path.read_text(encoding="utf-8-sig"), object_pairs_hook=_unique_object)
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise ProvenanceError("data provenance file is not valid UTF-8 JSON") from exc
    return validate_document(document, environment)


def default_synthetic(environment: str, checked_at: str) -> dict:
    """Create the package's safe default: no production data is copied."""
    _require(environment in _ENVIRONMENTS, "unsupported non-production environment")
    _timestamp(checked_at, "checked_at")
    return {
        "schema_version": 1,
        "environment": environment,
        "classification": "synthetic",
        "source_ref": "generated-fixtures",
        "approval": {"status": "not_required", "reference": "", "approved_by": "", "approved_at": ""},
        "verification": {"status": "passed", "method": "package contains no business data; synthetic fixtures only",
                          "checked_by": "environmentctl", "checked_at": checked_at, "pii_scan": "passed"},
    }


def write(path: Path, document: dict) -> None:
    """Write a validated attestation without exposing business values."""
    validate_document(document, document["environment"])
    path.write_text(json.dumps(document, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
