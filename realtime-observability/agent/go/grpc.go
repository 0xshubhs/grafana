package agent

// gRPC client interfaces and types
// In production, these would be generated from the protobuf file

import (
	"context"

	"google.golang.org/grpc"
)

// TelemetryIngestorClient is the client API for TelemetryIngestor service
type TelemetryIngestorClient interface {
	StreamTelemetry(ctx context.Context, opts ...grpc.CallOption) (TelemetryIngestor_StreamTelemetryClient, error)
}

type telemetryIngestorClient struct {
	cc grpc.ClientConnInterface
}

// NewTelemetryIngestorClient creates a new client
func NewTelemetryIngestorClient(cc grpc.ClientConnInterface) TelemetryIngestorClient {
	return &telemetryIngestorClient{cc}
}

func (c *telemetryIngestorClient) StreamTelemetry(ctx context.Context, opts ...grpc.CallOption) (TelemetryIngestor_StreamTelemetryClient, error) {
	stream, err := c.cc.NewStream(ctx, &grpc.StreamDesc{
		StreamName:    "StreamTelemetry",
		ClientStreams: true,
	}, "/telemetry.TelemetryIngestor/StreamTelemetry", opts...)
	if err != nil {
		return nil, err
	}
	return &telemetryIngestorStreamTelemetryClient{stream}, nil
}

// TelemetryIngestor_StreamTelemetryClient is the client stream interface
type TelemetryIngestor_StreamTelemetryClient interface {
	Send(*TelemetryBatch) error
	CloseAndRecv() (*Ack, error)
	grpc.ClientStream
}

type telemetryIngestorStreamTelemetryClient struct {
	grpc.ClientStream
}

func (x *telemetryIngestorStreamTelemetryClient) Send(m *TelemetryBatch) error {
	return x.ClientStream.SendMsg(m)
}

func (x *telemetryIngestorStreamTelemetryClient) CloseAndRecv() (*Ack, error) {
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	m := new(Ack)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Message types (normally generated from proto)

type Ack struct {
	Ok bool
}

type TelemetryBatch struct {
	Service  string
	Instance string
	Metrics  []*Metric
}

type Metric struct {
	Name    string
	Labels  map[string]string
	Samples []*MetricSample
}

type MetricSample struct {
	TimestampNs uint64
	Value       isMetricSample_Value
}

type isMetricSample_Value interface {
	isMetricSample_Value()
}

type MetricSample_Gauge struct {
	Gauge float64
}

type MetricSample_Counter struct {
	Counter uint64
}

type MetricSample_Histogram struct {
	Histogram *HistogramProto
}

func (*MetricSample_Gauge) isMetricSample_Value()     {}
func (*MetricSample_Counter) isMetricSample_Value()   {}
func (*MetricSample_Histogram) isMetricSample_Value() {}

type HistogramProto struct {
	Bounds []float64
	Counts []uint64
}
