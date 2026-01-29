package main

import (
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	agent "github.com/yourorg/agent"
)

func main() {
	// Configure the agent from environment
	config := agent.DefaultConfig()
	config.ServiceName = getEnv("SERVICE_NAME", "mac-local-service")
	config.InstanceID = getEnv("INSTANCE_ID", "mac-node-2")
	config.AggregatorAddr = getEnv("AGGREGATOR_ADDR", "localhost:9000")
	config.APIKey = getEnv("API_KEY", "dev-key-123")

	pushInterval, _ := strconv.Atoi(getEnv("PUSH_INTERVAL_MS", "20"))
	config.PushInterval = time.Duration(pushInterval) * time.Millisecond

	log.Printf("🚀 Starting telemetry agent")
	log.Printf("   Service: %s", config.ServiceName)
	log.Printf("   Instance: %s", config.InstanceID)
	log.Printf("   Aggregator: %s", config.AggregatorAddr)
	log.Printf("   Push Interval: %v", config.PushInterval)

	// Create and connect agent
	a, err := agent.NewAgent(config)
	if err != nil {
		log.Fatalf("❌ Failed to create agent: %v", err)
	}

	// Retry connection with backoff
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		if err := a.Connect(); err != nil {
			log.Printf("⏳ Connection attempt %d/%d failed: %v", i+1, maxRetries, err)
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		break
	}

	// Start the agent
	a.Start()
	log.Printf("✅ Agent connected and streaming telemetry")

	// Simulate realistic workload
	go simulateWorkload(a)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 Shutting down agent...")
	a.Stop()
	log.Println("👋 Agent stopped")
}

// simulateWorkload generates realistic metrics continuously
func simulateWorkload(a *agent.Agent) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	requestCount := uint64(0)
	errorCount := uint64(0)
	startTime := time.Now()

	for range ticker.C {
		// Simulate request with variable latency
		done := a.TrackRequest()

		// Simulate processing time (5-150ms with occasional spikes)
		baseLatency := 10 + rand.Intn(40)
		if rand.Float32() < 0.05 {
			baseLatency += rand.Intn(200) // 5% chance of slow request
		}
		time.Sleep(time.Duration(baseLatency) * time.Millisecond)

		done()
		requestCount++

		// Simulate occasional errors (3-5%)
		if rand.Float32() < 0.04 {
			errorTypes := []string{"timeout", "connection", "validation", "internal"}
			a.RecordError(errorTypes[rand.Intn(len(errorTypes))])
			errorCount++
		}

		// Calculate and emit derived metrics
		elapsed := time.Since(startTime).Seconds()
		if elapsed > 0 {
			rps := float64(requestCount) / elapsed
			errorRate := float64(errorCount) / float64(requestCount+1)

			a.SetGauge("rps", rps)
			a.SetGauge("error_rate", errorRate)
		}

		// Simulate system metrics
		a.SetGauge("cpu_usage", 15+rand.Float64()*70)
		a.SetGauge("memory_mb", 80+rand.Float64()*150)
		a.SetGauge("connections", float64(5+rand.Intn(45)))
		a.SetGauge("goroutines", float64(20+rand.Intn(100)))

		// Simulate latency percentiles (computed from histogram)
		a.SetGauge("latency_p50", float64(15+rand.Intn(20)))
		a.SetGauge("latency_p95", float64(50+rand.Intn(50)))
		a.SetGauge("latency_p99", float64(100+rand.Intn(150)))

		// Increment counters
		a.IncCounter("requests_total")
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
