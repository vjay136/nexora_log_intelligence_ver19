package controlplane

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MetricDescriptor struct {
	Name   string   `json:"name"`
	Group  string   `json:"group"`
	Type   string   `json:"type"`
	Help   string   `json:"help"`
	Labels []string `json:"labels"`
	Unit   string   `json:"unit,omitempty"`
	Source string   `json:"source"`
}

type metricCatalog struct {
	SchemaVersion             string             `json:"schema_version"`
	Descriptors               []MetricDescriptor `json:"descriptors"`
	DescriptorCount           int                `json:"descriptor_count"`
	AddressableSeriesFloor    int                `json:"addressable_series_floor"`
	AddressabilityExplanation string             `json:"addressability_explanation"`
}

var controllerMetricDescriptors = []MetricDescriptor{
	{Name: "nli_controller_cpu_cores", Group: "controller", Type: "gauge", Help: "Logical CPUs available to the controller process.", Unit: "cores", Source: "runtime.NumCPU"},
	{Name: "nli_controller_memory_alloc_bytes", Group: "controller", Type: "gauge", Help: "Bytes allocated and currently in use by the Go heap.", Unit: "bytes", Source: "runtime.MemStats.Alloc"},
	{Name: "nli_controller_memory_sys_bytes", Group: "controller", Type: "gauge", Help: "Bytes obtained from the operating system by the Go runtime.", Unit: "bytes", Source: "runtime.MemStats.Sys"},
	{Name: "nli_controller_goroutines", Group: "controller", Type: "gauge", Help: "Current number of Go goroutines.", Unit: "goroutines", Source: "runtime.NumGoroutine"},
	{Name: "nli_controller_gc_pause_seconds_total", Group: "controller", Type: "counter", Help: "Cumulative garbage collection pause time.", Unit: "seconds", Source: "runtime.MemStats.PauseTotalNs"},
	{Name: "nli_controller_gc_cycles_total", Group: "controller", Type: "counter", Help: "Completed garbage collection cycles.", Unit: "cycles", Source: "runtime.MemStats.NumGC"},
	{Name: "nli_controller_uptime_seconds", Group: "controller", Type: "gauge", Help: "Seconds since controller process startup.", Unit: "seconds", Source: "controller startup clock"},
}

var clientMetricDescriptors = []MetricDescriptor{
	{Name: "nli_client_cpu_percent", Group: "client", Type: "gauge", Help: "Latest reported host CPU utilization.", Labels: []string{"node_id", "hostname"}, Unit: "percent", Source: "latest heartbeat with host_metrics_available"},
	{Name: "nli_client_memory_bytes", Group: "client", Type: "gauge", Help: "Latest reported host memory usage.", Labels: []string{"node_id", "hostname"}, Unit: "bytes", Source: "latest heartbeat with host_metrics_available"},
	{Name: "nli_client_memory_percent", Group: "client", Type: "gauge", Help: "Latest reported host memory utilization.", Labels: []string{"node_id", "hostname"}, Unit: "percent", Source: "latest heartbeat with host_metrics_available"},
	{Name: "nli_client_load_1m", Group: "client", Type: "gauge", Help: "Latest reported one-minute system load.", Labels: []string{"node_id", "hostname"}, Source: "latest heartbeat with host_metrics_available"},
	{Name: "nli_client_load_5m", Group: "client", Type: "gauge", Help: "Latest reported five-minute system load.", Labels: []string{"node_id", "hostname"}, Source: "latest heartbeat with host_metrics_available"},
	{Name: "nli_client_load_15m", Group: "client", Type: "gauge", Help: "Latest reported fifteen-minute system load.", Labels: []string{"node_id", "hostname"}, Source: "latest heartbeat with host_metrics_available"},
	{Name: "nli_client_parse_latency_ms", Group: "client", Type: "gauge", Help: "Latest reported parse latency.", Labels: []string{"node_id", "hostname"}, Unit: "milliseconds", Source: "latest heartbeat"},
	{Name: "nli_stream_read_lines_per_second", Group: "client", Type: "gauge", Help: "Latest source-reported read rate.", Labels: []string{"node_id", "source", "phase"}, Unit: "lines/second", Source: "latest stream telemetry"},
	{Name: "nli_stream_normalized_per_second", Group: "client", Type: "gauge", Help: "Latest source-reported normalization rate.", Labels: []string{"node_id", "source", "phase"}, Unit: "records/second", Source: "latest stream telemetry"},
	{Name: "nli_stream_quarantined_per_second", Group: "client", Type: "gauge", Help: "Latest source-reported quarantine rate.", Labels: []string{"node_id", "source", "phase"}, Unit: "records/second", Source: "latest stream telemetry"},
	{Name: "nli_stream_bytes_per_second", Group: "client", Type: "gauge", Help: "Latest source-reported byte rate.", Labels: []string{"node_id", "source", "phase"}, Unit: "bytes/second", Source: "latest stream telemetry"},
	{Name: "nli_stream_read_lines_total", Group: "client", Type: "counter", Help: "Latest source cumulative lines read.", Labels: []string{"node_id", "source"}, Unit: "lines", Source: "latest stream telemetry"},
	{Name: "nli_stream_normalized_total", Group: "client", Type: "counter", Help: "Latest source cumulative normalized records.", Labels: []string{"node_id", "source"}, Unit: "records", Source: "latest stream telemetry"},
	{Name: "nli_stream_quarantined_total", Group: "client", Type: "counter", Help: "Latest source cumulative quarantined records.", Labels: []string{"node_id", "source"}, Unit: "records", Source: "latest stream telemetry"},
	{Name: "nli_stream_errors_total", Group: "client", Type: "counter", Help: "Latest source cumulative errors.", Labels: []string{"node_id", "source"}, Unit: "errors", Source: "latest stream telemetry"},
	{Name: "nli_stream_queue_depth", Group: "client", Type: "gauge", Help: "Latest reported source queue depth.", Labels: []string{"node_id", "source"}, Unit: "records", Source: "latest stream telemetry"},
	{Name: "nli_stream_queue_capacity", Group: "client", Type: "gauge", Help: "Latest reported source queue capacity.", Labels: []string{"node_id", "source"}, Unit: "records", Source: "latest stream telemetry"},
	{Name: "nli_stream_source_workers", Group: "client", Type: "gauge", Help: "Latest reported source worker count.", Labels: []string{"node_id", "source"}, Unit: "workers", Source: "latest stream telemetry"},
	{Name: "nli_stream_backpressure", Group: "client", Type: "gauge", Help: "Latest backpressure state, one active and zero inactive.", Labels: []string{"node_id", "source"}, Unit: "boolean", Source: "latest stream telemetry"},
}

func buildMetricCatalog() metricCatalog {
	all := append(append([]MetricDescriptor{}, controllerMetricDescriptors...), clientMetricDescriptors...)
	return metricCatalog{SchemaVersion: "1", Descriptors: all, DescriptorCount: len(all), AddressableSeriesFloor: len(clientMetricDescriptors) * 1000, AddressabilityExplanation: "Each client descriptor accepts observed node_id values. The floor is dimensional addressability, not a count of live clients or measurements. Only live measurements are exported."}
}
func (s *Server) handleMetricCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		jsonError(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	jsonOK(w, buildMetricCatalog())
}
func (s *Server) handleOpenMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		jsonError(w, "GET required", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/openmetrics-text; version=1.0.0; charset=utf-8")
	all := append(append([]MetricDescriptor{}, controllerMetricDescriptors...), clientMetricDescriptors...)
	for _, d := range all {
		fmt.Fprintln(w, "# HELP "+d.Name+" "+d.Help)
		fmt.Fprintln(w, "# TYPE "+d.Name+" "+d.Type)
	}
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	emit := func(name string, value float64, labels ...string) {
		fmt.Fprint(w, name, formatLabels(labels), " ", strconv.FormatFloat(value, 'g', -1, 64))
		fmt.Fprintln(w)
	}
	emitUint := func(name string, value uint64, labels ...string) {
		fmt.Fprint(w, name, formatLabels(labels), " ", strconv.FormatUint(value, 10))
		fmt.Fprintln(w)
	}
	emit("nli_controller_cpu_cores", float64(runtime.NumCPU()))
	emit("nli_controller_memory_alloc_bytes", float64(memory.Alloc))
	emit("nli_controller_memory_sys_bytes", float64(memory.Sys))
	emit("nli_controller_goroutines", float64(runtime.NumGoroutine()))
	emit("nli_controller_gc_pause_seconds_total", float64(memory.PauseTotalNs)/float64(time.Second))
	emit("nli_controller_gc_cycles_total", float64(memory.NumGC))
	emit("nli_controller_uptime_seconds", time.Since(s.startedAt).Seconds())
	nodes := s.fleet.All()
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	for _, node := range nodes {
		if node.Metrics == nil {
			continue
		}
		labels := []string{"node_id", node.ID, "hostname", node.Hostname}
		emit("nli_client_parse_latency_ms", node.Metrics.ParseLatencyMs, labels...)
		if node.Metrics.HostMetricsAvailable {
			values := map[string]float64{"nli_client_cpu_percent": node.Metrics.CPUPercent, "nli_client_memory_bytes": float64(node.Metrics.MemoryBytes), "nli_client_memory_percent": node.Metrics.MemoryPercent, "nli_client_load_1m": node.Metrics.Load1, "nli_client_load_5m": node.Metrics.Load5, "nli_client_load_15m": node.Metrics.Load15}
			for name, value := range values {
				emit(name, value, labels...)
			}
		}
	}
	files := s.streamTelemetry.Snapshot()
	sort.Slice(files, func(i, j int) bool {
		if files[i].NodeID == files[j].NodeID {
			return files[i].Source < files[j].Source
		}
		return files[i].NodeID < files[j].NodeID
	})
	for _, file := range files {
		labels := []string{"node_id", file.NodeID, "source", file.Source}
		phaseLabels := append(append([]string{}, labels...), "phase", file.Phase)
		emit("nli_stream_read_lines_per_second", file.ReadLinesPerSecond, phaseLabels...)
		emit("nli_stream_normalized_per_second", file.NormalizedPerSecond, phaseLabels...)
		emit("nli_stream_quarantined_per_second", file.QuarantinedPerSecond, phaseLabels...)
		emit("nli_stream_bytes_per_second", file.BytesPerSecond, phaseLabels...)
		emitUint("nli_stream_read_lines_total", file.ReadLines, labels...)
		emitUint("nli_stream_normalized_total", file.Normalized, labels...)
		emitUint("nli_stream_quarantined_total", file.Quarantined, labels...)
		emitUint("nli_stream_errors_total", file.Errors, labels...)
		emit("nli_stream_queue_depth", float64(file.QueueDepth), labels...)
		emit("nli_stream_queue_capacity", float64(file.QueueCapacity), labels...)
		emit("nli_stream_source_workers", float64(file.SourceWorkers), labels...)
		backpressure := 0.0
		if file.Backpressure {
			backpressure = 1
		}
		emit("nli_stream_backpressure", backpressure, labels...)
	}
	fmt.Fprintln(w, "# EOF")
}
func formatLabels(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels)/2)
	for i := 0; i+1 < len(labels); i += 2 {
		parts = append(parts, labels[i]+"="+strconv.Quote(labels[i+1]))
	}
	return "{" + strings.Join(parts, ",") + "}"
}
