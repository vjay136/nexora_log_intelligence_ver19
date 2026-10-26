#!/usr/bin/env python3
"""Verify the UPES/GPRS release guide and diagram are shipped everywhere."""

from __future__ import annotations

import argparse
from pathlib import Path


DOC = "V19_UPES_GPRS_ARCHITECTURE_AND_ACCEPTANCE.md"
DIAGRAM = "NLI_V19_UPES_GPRS_ARCHITECTURE.svg"


def fail(message: str) -> None:
    raise SystemExit(f"UPES/GPRS release-doc packaging: FAIL: {message}")


def validate(root: Path, output: Path, client: Path, controller: Path) -> None:
    sources = {
        DOC: root / "docs" / DOC,
        DIAGRAM: root / "docs" / "architecture" / DIAGRAM,
    }
    for name, source in sources.items():
        if not source.is_file() or source.stat().st_size == 0:
            fail(f"source artifact is missing or empty: {source}")
        for target_dir in (output, client, controller):
            target = target_dir / name
            if not target.is_file() or target.read_bytes() != source.read_bytes():
                fail(f"{name} is missing or differs in {target_dir}")

    guide = sources[DOC].read_text(encoding="utf-8")
    for claim in (
        "19.0.0-6",
        "does not accept or store a raw record or source path",
        "does not claim a",
        "submitted_checksum",
        "approved_checksum",
        "independent rollout approval",
        "does not declare the release build",
    ):
        if claim not in guide:
            fail(f"guide does not document the expected release boundary: {claim!r}")

    for relative in (
        "packaging/build-rpms.sh",
        "packaging/rpm/nli-client.spec",
        "packaging/rpm/nli-controller-agent.spec",
        "packaging/build-release.ps1",
        "scripts/release/build.sh",
        "scripts/release/generate-evidence.py",
        "scripts/release/verify-artifacts.sh",
        "tests/release/package-infra.sh",
    ):
        source = (root / relative).read_text(encoding="utf-8")
        for name in sources:
            if name not in source:
                fail(f"{relative} does not reference packaged artifact {name}")

    if f"![UPES and governed parser-candidate lifecycle]({DIAGRAM})" not in guide:
        fail("guide diagram link must resolve when the archive or RPM docs are read")

    for relative in (
        "CHANGELOG.md",
        "docs/release/README.md",
        "docs/release/V19_MIGRATION_AND_ROLLBACK.md",
        "packaging/rpm/nli-client.spec",
        "packaging/rpm/nli-controller-agent.spec",
    ):
        if "19.0.0-6" not in (root / relative).read_text(encoding="utf-8"):
            fail(f"{relative} does not identify the current release")

    release_env = (root / "packaging" / "release.env").read_text(encoding="utf-8")
    if "NLI_VERSION=19.0.0" not in release_env or "NLI_RELEASE=6" not in release_env:
        fail("packaging release identity is not 19.0.0-6")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--client", required=True, type=Path)
    parser.add_argument("--controller", required=True, type=Path)
    args = parser.parse_args()
    validate(args.root.resolve(), args.output.resolve(), args.client.resolve(), args.controller.resolve())
    print("UPES/GPRS release-doc packaging: PASS")


if __name__ == "__main__":
    main()
