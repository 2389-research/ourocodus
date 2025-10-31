package nats

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Global singleton for metrics to prevent duplicate registration
var (
	globalMetrics     *metricsCollector
	globalMetricsOnce sync.Once
)

// metricsCollector collects Prometheus metrics for the NATS client.
type metricsCollector struct {
	config *ClientConfig

	// Connection metrics
	connectionUp prometheus.Gauge
	reconnects   prometheus.Counter

	// Publish metrics
	messagesPublished *prometheus.CounterVec
	publishErrors     *prometheus.CounterVec
	publishLatency    *prometheus.HistogramVec

	// Subscribe metrics
	messagesReceived *prometheus.CounterVec
	handlerErrors    *prometheus.CounterVec
	handlerDuration  *prometheus.HistogramVec

	// Request metrics
	requests        *prometheus.CounterVec
	requestErrors   *prometheus.CounterVec
	requestDuration *prometheus.HistogramVec
}

// newMetricsCollector creates a new metrics collector.
// Uses singleton pattern to prevent duplicate Prometheus registration.
func newMetricsCollector(config *ClientConfig) *metricsCollector {
	if !config.MetricsEnabled {
		return &metricsCollector{config: config}
	}

	// Use global singleton to prevent duplicate registration with Prometheus
	globalMetricsOnce.Do(func() {
		globalMetrics = &metricsCollector{
			config: config,
		}
		globalMetrics.registerMetrics()
	})

	return globalMetrics
}

// registerMetrics registers all Prometheus metrics.
// This should only be called once via the global singleton pattern.
func (m *metricsCollector) registerMetrics() {
	namespace := m.config.MetricsNamespace
	subsystem := m.config.MetricsSubsystem

	// Connection metrics
	m.connectionUp = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "connection_up",
		Help:      "Whether the connection to NATS is up (1) or down (0)",
	})

	m.reconnects = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Subsystem: subsystem,
		Name:      "reconnects_total",
		Help:      "Total number of reconnection attempts",
	})

	// Publish metrics
	m.messagesPublished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "messages_published_total",
			Help:      "Total number of messages published",
		},
		[]string{"subject", "status"},
	)

	m.publishErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "publish_errors_total",
			Help:      "Total number of publish errors",
		},
		[]string{"subject", "error_type"},
	)

	m.publishLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "publish_latency_seconds",
			Help:      "Publish operation latency in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"subject"},
	)

	// Subscribe metrics
	m.messagesReceived = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "messages_received_total",
			Help:      "Total number of messages received",
		},
		[]string{"subject"},
	)

	m.handlerErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "handler_errors_total",
			Help:      "Total number of handler errors",
		},
		[]string{"subject", "error_type"},
	)

	m.handlerDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "handler_duration_seconds",
			Help:      "Handler execution duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"subject"},
	)

	// Request metrics
	m.requests = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "requests_total",
			Help:      "Total number of requests sent",
		},
		[]string{"subject", "status"},
	)

	m.requestErrors = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "request_errors_total",
			Help:      "Total number of request errors",
		},
		[]string{"subject", "error_type"},
	)

	m.requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "request_duration_seconds",
			Help:      "Request operation duration in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"subject"},
	)

	// Initialize connection as up
	m.connectionUp.Set(1)
}

// recordPublish records a publish operation.
func (m *metricsCollector) recordPublish(subject string, duration time.Duration, err error) {
	if !m.config.MetricsEnabled {
		return
	}

	// Normalize subject to prevent high cardinality
	normalizedSubject := normalizeSubject(subject)

	if err == nil {
		m.messagesPublished.WithLabelValues(normalizedSubject, "success").Inc()
	} else {
		m.messagesPublished.WithLabelValues(normalizedSubject, "error").Inc()
		errorType := "unknown"
		if IsTransientError(err) {
			errorType = "transient"
		} else if IsPermanentError(err) {
			errorType = "permanent"
		}
		m.publishErrors.WithLabelValues(normalizedSubject, errorType).Inc()
	}

	m.publishLatency.WithLabelValues(normalizedSubject).Observe(duration.Seconds())
}

// recordMessageReceived records a received message.
func (m *metricsCollector) recordMessageReceived(subject string, duration time.Duration, err error) {
	if !m.config.MetricsEnabled {
		return
	}

	normalizedSubject := normalizeSubject(subject)

	m.messagesReceived.WithLabelValues(normalizedSubject).Inc()
	m.handlerDuration.WithLabelValues(normalizedSubject).Observe(duration.Seconds())

	if err != nil {
		errorType := "handler_error"
		m.handlerErrors.WithLabelValues(normalizedSubject, errorType).Inc()
	}
}

// recordRequest records a request operation.
func (m *metricsCollector) recordRequest(subject string, duration time.Duration, err error) {
	if !m.config.MetricsEnabled {
		return
	}

	normalizedSubject := normalizeSubject(subject)

	if err == nil {
		m.requests.WithLabelValues(normalizedSubject, "success").Inc()
	} else {
		m.requests.WithLabelValues(normalizedSubject, "error").Inc()
		errorType := "unknown"
		if IsTransientError(err) {
			errorType = "transient"
		} else if IsPermanentError(err) {
			errorType = "permanent"
		}
		m.requestErrors.WithLabelValues(normalizedSubject, errorType).Inc()
	}

	m.requestDuration.WithLabelValues(normalizedSubject).Observe(duration.Seconds())
}

// normalizeSubject normalizes a subject to prevent high cardinality.
// For example, "sessions.abc123.events" becomes "sessions.*.events"
func normalizeSubject(subject string) string {
	// TODO: Implement proper subject normalization
	// For now, return the subject as-is
	return subject
}
