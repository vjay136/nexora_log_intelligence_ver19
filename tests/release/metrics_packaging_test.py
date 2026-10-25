#!/usr/bin/env python3
"""Deterministic static validation for the operator metrics RPM payload."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path


REQUIRED = {
    "README.md",
    "metric-catalog.json",
    "prometheus-scrape.yml",
    "grafana-dashboard.json",
}


def fail(message: str) -> None:
    raise SystemExit(f"metrics packaging validation: FAIL: {message}")


def names_from_source(path: Path, source_start: str, source_end: str) -> set[str]:
    source = path.read_text(encoding="utf-8")
    try:
        source = source.split(source_start, 1)[1].split(source_end, 1)[0]
    except IndexError:
        fail(f"could not find metric source section in {path}")
    return set(re.findall(r'"(nli_[a-z0-9_]+)"', source))


def validate(root: Path, stage: Path) -> None:
    for directory in (root / "metrics", stage):
        missing = REQUIRED - {path.name for path in directory.iterdir() if path.is_file()}
        if missing:
            fail(f"{directory} is missing required files: {', '.join(sorted(missing))}")

    for name in sorted(REQUIRED):
        source = root / "metrics" / name
        packaged = stage / name
        if source.read_bytes() != packaged.read_bytes():
            fail(f"staged metrics artifact differs from source: {name}")

    catalog = json.loads((stage / "metric-catalog.json").read_text(encoding="utf-8"))
    dashboard = json.loads((stage / "grafana-dashboard.json").read_text(encoding="utf-8"))
    controller_descriptors = catalog.get("descriptors", [])
    client_descriptors = catalog.get("client_descriptors", [])
    if catalog.get("schema_version") != "1":
        fail("catalog schema_version must be 1")
    if catalog.get("descriptor_count") != len(controller_descriptors):
        fail("descriptor_count must match the controller catalog descriptor list")
    if len(controller_descriptors) != 26 or len(client_descriptors) != 8:
        fail("catalog must contain 26 controller and 8 client descriptors")
    if catalog.get("addressable_series_floor") != len([d for d in controller_descriptors if d["group"] == "client"]) * 1000:
        fail("addressable_series_floor must be derived from client descriptor count x 1,000")
    if "not a count of live" not in catalog.get("addressability_explanation", ""):
        fail("catalog must distinguish addressability from live samples")

    metrics_dir = root / "internal" / "agent" / "agent.go"
    client_source = metrics_dir.read_text(encoding="utf-8")
    try:
        client_section = client_source.split('mux.HandleFunc("/metrics"', 1)[1].split('mux.HandleFunc("/stream-snapshot"', 1)[0]
    except IndexError:
        fail("could not find client /metrics handler")
    actual_client = set(re.findall(r"nli_[a-z0-9_]+", client_section))
    catalog_client = {item["name"] for item in client_descriptors}
    if actual_client != catalog_client:
        fail(f"client catalog differs from /metrics source: missing={sorted(actual_client-catalog_client)} extra={sorted(catalog_client-actual_client)}")

    actual_controller = names_from_source(root / "internal" / "controlplane" / "metrics_catalog.go", "var controllerMetricDescriptors", "func buildMetricCatalog")
    catalog_controller = {item["name"] for item in controller_descriptors}
    if actual_controller != catalog_controller:
        fail(f"controller catalog differs from endpoint descriptors: missing={sorted(actual_controller-catalog_controller)} extra={sorted(catalog_controller-actual_controller)}")

    if dashboard.get("uid") != "nli-operations" or not dashboard.get("panels"):
        fail("Grafana dashboard must be importable and contain panels")
    known_metrics = catalog_client | catalog_controller
    expressions = [target.get("expr", "") for panel in dashboard["panels"] for target in panel.get("targets", [])]
    if not expressions or any(not any(name in expression for name in known_metrics) for expression in expressions):
        fail("every dashboard target must reference a catalogued metric")

    scrape = (stage / "prometheus-scrape.yml").read_text(encoding="utf-8")
    for required_text in ("job_name: nli-client", '"127.0.0.1:12380"', "job_name: nli-controller", "/api/v1/metrics/openmetrics", '"127.0.0.1:12382"', "credentials_file:"):
        if required_text not in scrape:
            fail(f"Prometheus example is missing {required_text!r}")


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--root", type=Path, required=True)
    parser.add_argument("--stage", type=Path, required=True)
    args = parser.parse_args()
    validate(args.root.resolve(), args.stage.resolve())
    print("metrics packaging validation: PASS")


if __name__ == "__main__":
    main()
