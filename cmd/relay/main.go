package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"github.com/tabular/relay/internal/gate"
	"github.com/tabular/relay/internal/metrics"
	"github.com/tabular/relay/internal/parser"
	"github.com/tabular/relay/internal/transformer"
	"github.com/tabular/relay/internal/updater"
	"github.com/tabular/relay/pkg/types"
)

func main() {
	// Load configuration
	config := loadConfig()
	
	// Initialize components
	relayMetrics := metrics.New()
	gateInstance := gate.New(config.WebSocket.BufferSize, config.WebSocket.HeartbeatInterval)
	parserInstance := parser.New()
	transformerInstance := transformer.New()
	updaterInstance := updater.New(config.STAG.URL, config.Batch.MaxSize, config.Batch.Timeout)
	
	// Start components
	gateInstance.Start()
	updaterInstance.Start()
	
	// Setup message processing pipeline
	go processMessages(gateInstance, parserInstance, transformerInstance, updaterInstance, relayMetrics)
	
	// Setup HTTP server
	router := setupRouter(gateInstance, relayMetrics)
	
	server := &http.Server{
		Addr:    config.Server.Host + ":" + config.Server.Port,
		Handler: router,
	}
	
	// Start server in goroutine
	go func() {
		log.Printf("Starting relay server on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()
	
	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	log.Println("Shutting down server...")
	
	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
	
	// Stop components
	gateInstance.Stop()
	updaterInstance.Stop()
	
	log.Println("Server exited")
}

func loadConfig() *types.Config {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/relay/")
	
	// Set defaults
	viper.SetDefault("server.port", "8080")
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("stag.url", "http://localhost:8081")
	viper.SetDefault("stag.timeout", "10s")
	viper.SetDefault("websocket.buffer_size", 1024)
	viper.SetDefault("websocket.heartbeat_interval", "30s")
	viper.SetDefault("batch.max_size", 5)
	viper.SetDefault("batch.timeout", "100ms")
	
	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			log.Println("No config file found, using defaults")
		} else {
			log.Printf("Error reading config file: %v", err)
		}
	}
	
	// Environment variable overrides
	viper.AutomaticEnv()
	viper.SetEnvPrefix("RELAY")
	
	var config types.Config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("Failed to unmarshal config: %v", err)
	}
	
	return &config
}

func setupRouter(gateInstance *gate.Gate, relayMetrics *metrics.Metrics) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	
	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":      "healthy",
			"timestamp":   time.Now().Unix(),
			"connections": gateInstance.GetActiveConnections(),
		})
	})
	
	// Metrics endpoint
	router.GET("/metrics", gin.WrapH(relayMetrics.Handler()))
	
	// WebSocket endpoint
	router.GET("/ws/streamkit", gin.WrapF(gateInstance.HandleWebSocket))
	
	// Status endpoint
	router.GET("/status", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"active_connections": gateInstance.GetActiveConnections(),
			"uptime":            time.Since(time.Now()).String(), // This would be tracked properly
		})
	})
	
	return router
}

func processMessages(
	gateInstance *gate.Gate,
	parserInstance *parser.Parser,
	transformerInstance *transformer.Transformer,
	updaterInstance *updater.Updater,
	relayMetrics *metrics.Metrics,
) {
	for msg := range gateInstance.Messages() {
		start := time.Now()
		
		// Parse packet
		parsedPacket, err := parserInstance.ParsePacket(msg.Packet)
		if err != nil {
			log.Printf("Failed to parse packet: %v", err)
			relayMetrics.RecordPacketError(msg.Packet.Type, "parse_error")
			continue
		}
		
		// Transform to event
		event, err := transformerInstance.Transform(*parsedPacket)
		if err != nil {
			log.Printf("Failed to transform packet: %v", err)
			relayMetrics.RecordPacketError(msg.Packet.Type, "transform_error")
			continue
		}
		
		// Process in updater
		if err := updaterInstance.ProcessEvent(*event); err != nil {
			log.Printf("Failed to process event: %v", err)
			relayMetrics.RecordPacketError(msg.Packet.Type, "update_error")
			continue
		}
		
		// Record success metrics
		relayMetrics.RecordPacket(msg.Packet.Type, "success")
		
		// Log processing time
		duration := time.Since(start)
		if duration > 10*time.Millisecond {
			log.Printf("Slow packet processing: %v for type %s", duration, msg.Packet.Type)
		}
	}
}