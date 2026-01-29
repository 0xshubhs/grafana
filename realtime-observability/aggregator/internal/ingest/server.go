package ingest

import (
	"io"
	"log"

	"github.com/yourorg/aggregator/internal/buffer"
	"github.com/yourorg/aggregator/internal/ws"
)

// Server implements the TelemetryIngestor gRPC service
type Server struct {
	UnimplementedTelemetryIngestorServer
	registry *buffer.Registry
	hub      *ws.Hub
}

// NewServer creates a new ingest server
func NewServer(registry *buffer.Registry, hub *ws.Hub) *Server {
	return &Server{
		registry: registry,
		hub:      hub,
	}
}

// StreamTelemetry handles the bidirectional streaming RPC
func (s *Server) StreamTelemetry(stream TelemetryIngestor_StreamTelemetryServer) error {
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&Ack{Ok: true})
		}
		if err != nil {
			log.Printf("Error receiving batch: %v", err)
			return err
		}

		// Process each metric in the batch
		for _, metric := range batch.Metrics {
			s.processMetric(batch.Service, batch.Instance, metric)
		}

		// Notify hub of new data for real-time streaming
		s.hub.NotifyUpdate(batch.Service)
	}
}

// processMetric routes metrics to appropriate ring buffers
func (s *Server) processMetric(service, instance string, metric *Metric) {
	for _, sample := range metric.Samples {
		ts := int64(sample.TimestampNs)

		switch v := sample.Value.(type) {
		case *MetricSample_Gauge:
			ring := s.registry.GetRing(service, metric.Name)
			ring.Push(buffer.Sample{
				Ts:  ts,
				Val: v.Gauge,
			})

		case *MetricSample_Counter:
			ring := s.registry.GetCounterRing(service, metric.Name)
			ring.Push(buffer.Sample{
				Ts:  ts,
				Val: float64(v.Counter),
			})

		case *MetricSample_Histogram:
			ring := s.registry.GetHistogramRing(service, metric.Name)
			ring.Push(buffer.HistogramData{
				Ts:     ts,
				Bounds: v.Histogram.Bounds,
				Counts: v.Histogram.Counts,
			})
		}
	}
}
