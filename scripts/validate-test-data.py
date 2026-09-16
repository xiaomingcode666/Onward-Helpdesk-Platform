#!/usr/bin/env python3
"""Validate the non-production data provenance attestation before import."""

from __future__ import annotations

import argparse
import json
from pathlib import Path
import sys

from data_provenance import ProvenanceError, read


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", required=True, type=Path)
    parser.add_argument("--environment", required=True, choices=("integration", "staging"))
    args = parser.parse_args(argv)
    try:
        # Keep the original path so read() can reject symlinked attestations.
        summary = read(args.manifest, args.environment)
    except (ProvenanceError, OSError) as exc:
        print(f"test-data: {exc}", file=sys.stderr)
        return 2
    print(json.dumps({"valid": True, **summary}, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
