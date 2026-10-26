# NLI metrics kit

This directory is installed by both RPMs at `/usr/share/nli/metrics`. It contains the metric catalog, a Prometheus scrape example, and an importable Grafana dashboard.

## Endpoints

- Client agent: `GET /metrics` on the address set by `metrics_address` in `/etc/nli/nli.json` (default `127.0.0.1:12380`). It exports thirteen unlabeled counters and gauges for the local agent.
- Controller catalog: authenticated `GET /api/v1/metrics/catalog` on the controller. It returns the machine-readable descriptors and dimensional addressability note.
- Controller samples: authenticated `GET /api/v1/metrics/openmetrics` on the controller. It emits runtime metrics and observed client heartbeat and stream telemetry samples using OpenMetrics text format.

The controller endpoints require a bearer token with telemetry read permission. Configure Prometheus with a protected `credentials_file`; do not put a token in this example or in a dashboard. The default controller listener is loopback HTTP at `127.0.0.1:12382`. For remote scraping, configure controller TLS and retain certificate verification, or scrape locally and use Prometheus `remote_write`.

The client metrics listener is loopback-only. Run a Prometheus agent on each client host to scrape `127.0.0.1:12380`, then forward samples with `remote_write`. The included scrape example demonstrates the local target and a colocated controller target; adjust file paths and addresses for your deployment.

## Capacity and sample behavior

The controller catalog describes 19 client telemetry metric families. Its addressable series floor is the descriptor count multiplied by 1,000 possible observed node identities. This is dimensional addressability, not a promise of 1,000 connected clients, 1,000 live series per family, or any minimum sample count. Stream metrics also vary by observed source (and, for rate metrics, phase). The controller exports only samples backed by a current client heartbeat or stream telemetry snapshot; it does not synthesize zero-valued client samples. Client `/metrics` exports thirteen process-local values, including five UPES metrics. UPES counters and its pending-queue gauge reset when the agent process restarts; the metrics contain no individual signature, raw record, or source path.

Metric names, types, units, labels, and sources are listed in `metric-catalog.json`. Import `grafana-dashboard.json` from Grafana's Dashboards UI and select the Prometheus data source when prompted.
