package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metrics for the relay
type Metrics struct {
	// Connection metrics
	ActiveConnections prometheus.Gauge
	TotalConnections  prometheus.Counter
	
	// Packet processing metrics
	PacketsProcessed *prometheus.CounterVec
	PacketErrors     *prometheus.CounterVec
	
	// Batch metrics
	BatchSize        prometheus.Histogram
	BatchProcessTime prometheus.Histogram
	
	// STAG integration metrics
	StagRequests     *prometheus.CounterVec
	StagLatency      prometheus.Histogram
	
	// Mesh diffing metrics
	MeshDeltaRatio   prometheus.Histogram
	TrackedMeshes    prometheus.Gauge
	
	// Compression metrics
	CompressionRatio prometheus.Histogram
	BytesSaved       prometheus.Counter
	CompressionTime  prometheus.Histogram
}

// New creates and registers all metrics
func New() *Metrics {
	m := &Metrics{
		ActiveConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "relay_connections_active",
			Help: "Number of active WebSocket connections",
		}),
		
		TotalConnections: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "relay_connections_total",
			Help: "Total number of WebSocket connections established",
		}),
		
		PacketsProcessed: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "relay_packets_processed_total",
				Help: "Total number of packets processed by type",
			},
			[]string{"type", "status"},
		),
		
		PacketErrors: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "relay_packet_errors_total",
				Help: "Total number of packet processing errors",
			},
			[]string{"type", "error"},
		),
		
		BatchSize: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "relay_batch_size",
			Help:    "Size of batches sent to STAG",
			Buckets: prometheus.LinearBuckets(1, 1, 10),
		}),
		
		BatchProcessTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "relay_batch_process_seconds",
			Help:    "Time taken to process and send batches",
			Buckets: prometheus.DefBuckets,
		}),
		
		StagRequests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "relay_stag_requests_total",
				Help: "Total number of requests sent to STAG",
			},
			[]string{"status"},
		),
		
		StagLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "relay_stag_request_duration_seconds",
			Help:    "Duration of STAG requests",
			Buckets: prometheus.DefBuckets,
		}),
		
		MeshDeltaRatio: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "relay_mesh_delta_ratio",
			Help:    "Ratio of delta size to full mesh size",
			Buckets: prometheus.LinearBuckets(0.1, 0.1, 10),
		}),
		
		TrackedMeshes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "relay_tracked_meshes",
			Help: "Number of meshes being tracked for diffing",
		}),
		
		CompressionRatio: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "relay_compression_ratio",
			Help:    "Draco compression ratio (compressed/original)",
			Buckets: prometheus.LinearBuckets(0.1, 0.1, 10),
		}),
		
		BytesSaved: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "relay_bytes_saved_total",
			Help: "Total bytes saved through compression",
		}),
		
		CompressionTime: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "relay_compression_duration_seconds",
			Help:    "Time taken to compress mesh data",
			Buckets: prometheus.DefBuckets,
		}),
	}
	
	// Register all metrics
	prometheus.MustRegister(
		m.ActiveConnections,
		m.TotalConnections,
		m.PacketsProcessed,
		m.PacketErrors,
		m.BatchSize,
		m.BatchProcessTime,
		m.StagRequests,
		m.StagLatency,
		m.MeshDeltaRatio,
		m.TrackedMeshes,
		m.CompressionRatio,
		m.BytesSaved,
		m.CompressionTime,
	)
	
	return m
}

// Handler returns the Prometheus metrics HTTP handler
func (m *Metrics) Handler() http.Handler {
	return promhttp.Handler()
}

// RecordConnection increments connection metrics
func (m *Metrics) RecordConnection() {
	m.TotalConnections.Inc()
	m.ActiveConnections.Inc()
}

// RecordDisconnection decrements active connections
func (m *Metrics) RecordDisconnection() {
	m.ActiveConnections.Dec()
}

// RecordPacket records packet processing metrics
func (m *Metrics) RecordPacket(packetType, status string) {
	m.PacketsProcessed.WithLabelValues(packetType, status).Inc()
}

// RecordPacketError records packet processing errors
func (m *Metrics) RecordPacketError(packetType, errorType string) {
	m.PacketErrors.WithLabelValues(packetType, errorType).Inc()
}

// RecordBatch records batch processing metrics
func (m *Metrics) RecordBatch(size int, duration float64) {
	m.BatchSize.Observe(float64(size))
	m.BatchProcessTime.Observe(duration)
}

// RecordStagRequest records STAG request metrics
func (m *Metrics) RecordStagRequest(status string, duration float64) {
	m.StagRequests.WithLabelValues(status).Inc()
	m.StagLatency.Observe(duration)
}

// RecordMeshDelta records mesh diffing metrics
func (m *Metrics) RecordMeshDelta(deltaRatio float64) {
	m.MeshDeltaRatio.Observe(deltaRatio)
}

// UpdateTrackedMeshes updates the number of tracked meshes
func (m *Metrics) UpdateTrackedMeshes(count int) {
	m.TrackedMeshes.Set(float64(count))
}

// RecordCompression records compression metrics
func (m *Metrics) RecordCompression(originalSize, compressedSize int, duration float64) {
	ratio := float64(compressedSize) / float64(originalSize)
	m.CompressionRatio.Observe(ratio)
	
	bytesSaved := originalSize - compressedSize
	if bytesSaved > 0 {
		m.BytesSaved.Add(float64(bytesSaved))
	}
	
	m.CompressionTime.Observe(duration)
}