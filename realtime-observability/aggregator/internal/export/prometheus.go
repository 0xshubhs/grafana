package export

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/yourorg/aggregator/internal/buffer"
)

// PrometheusExporter exports metrics to Prometheus
type PrometheusExporter struct {
	registry *buffer.Registry

	// Gauge metrics
	serviceLatency *prometheus.GaugeVec
	serviceRPS     *prometheus.GaugeVec
	serviceErrors  *prometheus.GaugeVec
	inflight       *prometheus.GaugeVec

	// Histogram metrics for latency percentiles
	latencyHistogram *prometheus.HistogramVec

	// Counter metrics
	requestsTotal *prometheus.CounterVec
	errorsTotal   *prometheus.CounterVec

	// System metrics
	activeConnections prometheus.Gauge
	bufferSize        *prometheus.GaugeVec
}

// NewPrometheusExporter creates a new Prometheus exporter
func NewPrometheusExporter(registry *buffer.Registry) *PrometheusExporter {
	return &PrometheusExporter{
		registry: registry,

		serviceLatency: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "service_latency_ms",
				Help: "Current service latency in milliseconds",
			},
			[]string{"service", "percentile"},
		),

		serviceRPS: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "service_requests_per_second",
				Help: "Current requests per second",
			},
			[]string{"service"},
		),

		serviceErrors: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "service_error_rate",
				Help: "Current error rate (0-1)",
			},
			[]string{"service"},
		),

		inflight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "service_inflight_requests",
				Help: "Current number of in-flight requests",
			},
			[]string{"service"},
		),

		latencyHistogram: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "service_latency_histogram_ms",
				Help:    "Service latency histogram in milliseconds",
				Buckets: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000},
			},
			[]string{"service"},
		),

		requestsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "service_requests_total",
				Help: "Total number of requests",
			},
			[]string{"service", "status"},
		),

		errorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "service_errors_total",
				Help: "Total number of errors",
			},
			[]string{"service", "type"},
		),

		activeConnections: prometheus.NewGauge(
			prometheus.GaugeOpts{
				Name: "aggregator_active_connections",
				Help: "Number of active WebSocket connections",
			},
		),

		bufferSize: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "aggregator_buffer_size",
				Help: "Current size of metric ring buffers",
			},
			[]string{"service", "metric"},
		),
	}
}

// Register registers all metrics with Prometheus
func (e *PrometheusExporter) Register() {
	prometheus.MustRegister(
		e.serviceLatency,
		e.serviceRPS,
		e.serviceErrors,
		e.inflight,
		e.latencyHistogram,
		e.requestsTotal,
		e.errorsTotal,
		e.activeConnections,
		e.bufferSize,
	)
}

// UpdateMetrics updates Prometheus metrics from the registry
func (e *PrometheusExporter) UpdateMetrics() {
	snapshot := e.registry.LatestSnapshot()

	for key, sample := range snapshot.Gauges {
		switch key.Name {
		case "latency_p50":
			e.serviceLatency.WithLabelValues(key.Service, "p50").Set(sample.Val)
		case "latency_p95":
			e.serviceLatency.WithLabelValues(key.Service, "p95").Set(sample.Val)
		case "latency_p99":
			e.serviceLatency.WithLabelValues(key.Service, "p99").Set(sample.Val)
		case "rps":
			e.serviceRPS.WithLabelValues(key.Service).Set(sample.Val)
		case "error_rate":
			e.serviceErrors.WithLabelValues(key.Service).Set(sample.Val)
		case "inflight":
			e.inflight.WithLabelValues(key.Service).Set(sample.Val)
		}
	}

	// Update histograms
	for key, hist := range snapshot.Histograms {
		if key.Name == "latency" {
			// Calculate percentiles from histogram
			p50, p95, p99 := calculatePercentiles(hist.Bounds, hist.Counts)
			e.serviceLatency.WithLabelValues(key.Service, "p50").Set(p50)
			e.serviceLatency.WithLabelValues(key.Service, "p95").Set(p95)
			e.serviceLatency.WithLabelValues(key.Service, "p99").Set(p99)
		}
	}
}

// calculatePercentiles calculates p50, p95, p99 from histogram data
func calculatePercentiles(bounds []float64, counts []uint64) (p50, p95, p99 float64) {
	if len(bounds) == 0 || len(counts) == 0 {
		return 0, 0, 0
	}

	var total uint64
	for _, c := range counts {
		total += c
	}

	if total == 0 {
		return 0, 0, 0
	}

	target50 := uint64(float64(total) * 0.50)
	target95 := uint64(float64(total) * 0.95)
	target99 := uint64(float64(total) * 0.99)

	var cumulative uint64
	for i, c := range counts {
		cumulative += c
		if p50 == 0 && cumulative >= target50 {
			if i < len(bounds) {
				p50 = bounds[i]
			}
		}
		if p95 == 0 && cumulative >= target95 {
			if i < len(bounds) {
				p95 = bounds[i]
			}
		}
		if p99 == 0 && cumulative >= target99 {
			if i < len(bounds) {
				p99 = bounds[i]
			}
		}
	}

	return p50, p95, p99
}

// SetActiveConnections updates the active connections gauge
func (e *PrometheusExporter) SetActiveConnections(count int) {
	e.activeConnections.Set(float64(count))
}

// RecordRequest records a request for a service
func (e *PrometheusExporter) RecordRequest(service, status string) {
	e.requestsTotal.WithLabelValues(service, status).Inc()
}

// RecordError records an error for a service
func (e *PrometheusExporter) RecordError(service, errorType string) {
	e.errorsTotal.WithLabelValues(service, errorType).Inc()
}

// ObserveLatency records a latency observation
func (e *PrometheusExporter) ObserveLatency(service string, latencyMs float64) {
	e.latencyHistogram.WithLabelValues(service).Observe(latencyMs)
}
