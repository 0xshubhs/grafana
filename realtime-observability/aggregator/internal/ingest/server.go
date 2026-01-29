package ingest

import (
	"io"
	"log"

	"github.com/yourorg/aggregator/internal/buffer"
	"github.com/yourorg/aggregator/internal/ws"
	pb "github.com/yourorg/telemetry/gen/proto"
	"google.golang.org/grpc"
)

// Server implements the TelemetryIngestor gRPC service
type Server struct {
	pb.UnimplementedTelemetryIngestorServer
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

// StreamTelemetry handles the client streaming RPC
func (s *Server) StreamTelemetry(stream grpc.ClientStreamingServer[pb.TelemetryBatch, pb.Ack]) error {
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.Ack{Ok: true})
		}
		if err != nil {
			log.Printf("Error receiving batch: %v", err)
			return err
		}

		log.Printf("Received batch from service=%s instance=%s metrics=%d",
			batch.Service, batch.Instance, len(batch.Metrics))

		// Process each metric in the batch
		for _, metric := range batch.Metrics {
			s.processMetric(batch.Service, batch.Instance, metric)
		}

		// Notify hub of new data for real-time streaming
		s.hub.NotifyUpdate(batch.Service)
	}
}

// processMetric routes metrics to appropriate ring buffers
func (s *Server) processMetric(service, instance string, metric *pb.Metric) {
	for _, sample := range metric.Samples {
		ts := int64(sample.TimestampNs)

		switch v := sample.Value.(type) {
		case *pb.MetricSample_Gauge:
			ring := s.registry.GetRing(service, metric.Name)
			ring.Push(buffer.Sample{
				Ts:  ts,
				Val: v.Gauge,
			})

		case *pb.MetricSample_Counter:
			ring := s.registry.GetCounterRing(service, metric.Name)
			ring.Push(buffer.Sample{
				Ts:  ts,
				Val: float64(v.Counter),
			})

		case *pb.MetricSample_Histogram:
			ring := s.registry.GetHistogramRing(service, metric.Name)
			ring.Push(buffer.HistogramData{
				Ts:     ts,
				Bounds: v.Histogram.Bounds,
				Counts: v.Histogram.Counts,
			})
		}
	}
}
