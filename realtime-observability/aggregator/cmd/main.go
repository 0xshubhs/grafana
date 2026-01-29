package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/yourorg/aggregator/internal/auth"
	"github.com/yourorg/aggregator/internal/buffer"
	"github.com/yourorg/aggregator/internal/export"
	"github.com/yourorg/aggregator/internal/ingest"
	"github.com/yourorg/aggregator/internal/ws"
	"google.golang.org/grpc"
)

func main() {
	log.Println("Starting aggregator...")

	// Initialize components
	registry := buffer.NewRegistry()
	hub := ws.NewHub(registry)
	authenticator := auth.NewAuthenticator()
	exporter := export.NewPrometheusExporter(registry)

	// Start WebSocket hub
	go hub.Run()

	// Start gRPC server
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(authenticator.UnaryInterceptor()),
		grpc.StreamInterceptor(authenticator.StreamInterceptor()),
	)
	ingestServer := ingest.NewServer(registry, hub)
	ingest.RegisterTelemetryIngestorServer(grpcServer, ingestServer)

	grpcLis, err := net.Listen("tcp", ":9000")
	if err != nil {
		log.Fatalf("Failed to listen on gRPC port: %v", err)
	}
	go func() {
		log.Println("gRPC server listening on :9000")
		if err := grpcServer.Serve(grpcLis); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	// Start WebSocket HTTP server
	wsMux := http.NewServeMux()
	wsMux.HandleFunc("/ws", hub.HandleWebSocket)
	wsMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	wsServer := &http.Server{
		Addr:    ":8080",
		Handler: wsMux,
	}
	go func() {
		log.Println("WebSocket server listening on :8080")
		if err := wsServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("WebSocket server error: %v", err)
		}
	}()

	// Start Prometheus metrics server
	metricsMux := http.NewServeMux()
	metricsMux.Handle("/metrics", promhttp.Handler())
	exporter.Register()
	metricsServer := &http.Server{
		Addr:    ":9100",
		Handler: metricsMux,
	}
	go func() {
		log.Println("Prometheus metrics server listening on :9100")
		if err := metricsServer.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Metrics server error: %v", err)
		}
	}()

	// Start broadcast loop
	go hub.StartBroadcastLoop(16 * time.Millisecond)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	grpcServer.GracefulStop()
	wsServer.Shutdown(ctx)
	metricsServer.Shutdown(ctx)

	log.Println("Aggregator stopped")
}
