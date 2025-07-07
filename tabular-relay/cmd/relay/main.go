package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"tabular-relay/relay/config"
	"tabular-relay/relay/gate/listener"
	"tabular-relay/relay/gate/manager"
	"tabular-relay/relay/logging"
	"tabular-relay/relay/parser"
	"tabular-relay/relay/transformer"
	"tabular-relay/relay/updater"
)

// RelayServer represents the main relay server
type RelayServer struct {
	config      *config.Config
	logger      *logging.Logger
	
	// Components
	manager     *manager.ConnectionManager
	listener    *listener.WebSocketListener
	parser      *parser.Parser
	transformer *transformer.Transformer
	updater     *updater.EventUpdater
	
	// Worker management
	workers     []Worker
	workerPool  chan chan []byte
	quit        chan bool
	
	// Context for graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

// Worker represents a processing worker
type Worker struct {
	ID          int
	WorkerPool  chan chan []byte
	JobChannel  chan []byte
	QuitChannel chan bool
	Logger      *zap.Logger
	Parser      *parser.Parser
	Transformer *transformer.Transformer
	Updater     *updater.EventUpdater
}

// NewWorker creates a new worker
func NewWorker(id int, workerPool chan chan []byte, logger *zap.Logger, 
	parser *parser.Parser, transformer *transformer.Transformer, updater *updater.EventUpdater) Worker {
	return Worker{
		ID:          id,
		WorkerPool:  workerPool,
		JobChannel:  make(chan []byte),
		QuitChannel: make(chan bool),
		Logger:      logger,
		Parser:      parser,
		Transformer: transformer,
		Updater:     updater,
	}
}

// Start starts the worker
func (w Worker) Start() {
	go func() {
		for {
			// Register worker in the worker pool
			w.WorkerPool <- w.JobChannel
			
			select {
			case job := <-w.JobChannel:
				// Process the job
				w.processMessage(job)
			case <-w.QuitChannel:
				// Stop the worker
				return
			}
		}
	}()
}

// Stop stops the worker
func (w Worker) Stop() {
	go func() {
		w.QuitChannel <- true
	}()
}

// processMessage processes a single message
func (w Worker) processMessage(message []byte) {
	startTime := time.Now()
	
	w.Logger.Debug("Processing message",
		zap.Int("worker_id", w.ID),
		zap.Int("message_size", len(message)),
	)
	
	// Parse the message
	packet, err := w.Parser.Parse(message)
	if err != nil {
		w.Logger.Error("Failed to parse message",
			zap.Int("worker_id", w.ID),
			zap.Error(err),
		)
		return
	}
	
	// Validate the packet
	if err := w.Parser.ValidatePacket(packet); err != nil {
		w.Logger.Error("Packet validation failed",
			zap.Int("worker_id", w.ID),
			zap.String("session_id", packet.Header.SessionID),
			zap.Error(err),
		)
		return
	}
	
	// Transform the packet
	events, err := w.Transformer.Transform(packet)
	if err != nil {
		w.Logger.Error("Failed to transform packet",
			zap.Int("worker_id", w.ID),
			zap.String("session_id", packet.Header.SessionID),
			zap.Error(err),
		)
		return
	}
	
	// Send events to updater
	for _, event := range events {
		w.Updater.ProcessEvent(event)
	}
	
	processingTime := time.Since(startTime)
	w.Logger.Debug("Message processed successfully",
		zap.Int("worker_id", w.ID),
		zap.String("session_id", packet.Header.SessionID),
		zap.Uint64("frame_number", packet.Header.FrameNumber),
		zap.Int("events_created", len(events)),
		zap.Duration("processing_time", processingTime),
	)
}

// NewRelayServer creates a new relay server
func NewRelayServer() (*RelayServer, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	// Initialize logger
	logger, err := logging.NewLogger(cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	
	logger.LogStartup("relay", map[string]interface{}{
		"config": cfg.String(),
	})
	
	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	
	// Initialize components
	connectionManager := manager.NewConnectionManager(cfg, logger.GetZapLogger())
	wsListener := listener.NewWebSocketListener(cfg, logger.GetZapLogger(), connectionManager)
	packetParser := parser.NewParser(logger.GetZapLogger())
	eventTransformer := transformer.NewTransformer(logger.GetZapLogger())
	eventUpdater := updater.NewEventUpdater(cfg, logger.GetZapLogger())
	
	// Create worker pool
	workerPool := make(chan chan []byte, cfg.WorkerThreads)
	
	server := &RelayServer{
		config:      cfg,
		logger:      logger,
		manager:     connectionManager,
		listener:    wsListener,
		parser:      packetParser,
		transformer: eventTransformer,
		updater:     eventUpdater,
		workerPool:  workerPool,
		quit:        make(chan bool),
		ctx:         ctx,
		cancel:      cancel,
	}
	
	// Initialize workers
	server.initializeWorkers()
	
	return server, nil
}

// initializeWorkers creates and starts worker goroutines
func (s *RelayServer) initializeWorkers() {
	s.workers = make([]Worker, s.config.WorkerThreads)
	
	for i := 0; i < s.config.WorkerThreads; i++ {
		worker := NewWorker(i, s.workerPool, s.logger.GetZapLogger(), 
			s.parser, s.transformer, s.updater)
		s.workers[i] = worker
		worker.Start()
		
		s.logger.Debug("Worker started",
			zap.Int("worker_id", i),
		)
	}
	
	s.logger.Info("Worker pool initialized",
		zap.Int("worker_count", s.config.WorkerThreads),
	)
}

// Start starts the relay server
func (s *RelayServer) Start() error {
	s.logger.Info("Starting tabular-relay server",
		zap.Int("port", s.config.Port),
		zap.Int("max_clients", s.config.MaxClients),
		zap.Int("worker_threads", s.config.WorkerThreads),
	)
	
	// Start components
	s.manager.Start()
	s.updater.Start()
	
	// Start message dispatcher
	go s.startMessageDispatcher()
	
	// Start statistics logger
	go s.startStatsLogger()
	
	// Start WebSocket listener (blocking)
	return s.listener.Start(s.ctx)
}

// startMessageDispatcher starts the message dispatcher goroutine
func (s *RelayServer) startMessageDispatcher() {
	messageQueue := s.manager.GetMessageQueue()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case message := <-messageQueue:
			// Get an available worker
			go func(msg []byte) {
				select {
				case <-s.ctx.Done():
					return
				case jobChannel := <-s.workerPool:
					jobChannel <- msg
				}
			}(message)
		}
	}
}

// startStatsLogger starts periodic statistics logging
func (s *RelayServer) startStatsLogger() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.logStatistics()
		}
	}
}

// logStatistics logs current system statistics
func (s *RelayServer) logStatistics() {
	// Connection stats
	connStats := s.manager.GetStats()
	s.logger.LogStats("connections", map[string]interface{}{
		"active_connections": connStats.ActiveConnections,
		"total_connections":  connStats.TotalConnections,
		"bytes_received":     connStats.BytesReceived,
		"bytes_sent":         connStats.BytesSent,
		"uptime":            time.Since(connStats.StartTime).String(),
	})
	
	// Updater stats
	updaterStats := s.updater.GetStats()
	s.logger.LogStats("updater", map[string]interface{}{
		"events_received":       updaterStats.EventsReceived,
		"events_processed":      updaterStats.EventsProcessed,
		"events_dropped":        updaterStats.EventsDropped,
		"batches_sent":          updaterStats.BatchesSent,
		"batches_successful":    updaterStats.BatchesSuccessful,
		"batches_failed":        updaterStats.BatchesFailed,
		"total_retries":         updaterStats.TotalRetries,
		"buffer_size":           s.updater.GetBufferSize(),
		"last_successful_send":  updaterStats.LastSuccessfulSend,
		"last_failed_send":      updaterStats.LastFailedSend,
	})
}

// Stop gracefully stops the relay server
func (s *RelayServer) Stop() error {
	s.logger.LogShutdown("relay", "graceful shutdown requested")
	
	// Cancel context to signal shutdown
	s.cancel()
	
	// Stop components in reverse order
	var wg sync.WaitGroup
	
	// Stop listener
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.listener.Stop(); err != nil {
			s.logger.Error("Error stopping listener", zap.Error(err))
		}
	}()
	
	// Stop workers
	for _, worker := range s.workers {
		worker.Stop()
	}
	
	// Stop connection manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.manager.Stop()
	}()
	
	// Stop updater
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.updater.Stop()
	}()
	
	// Wait for all components to stop
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	
	// Wait with timeout
	select {
	case <-done:
		s.logger.Info("All components stopped successfully")
	case <-time.After(30 * time.Second):
		s.logger.Warn("Shutdown timeout reached, forcing exit")
	}
	
	// Sync logger
	if err := s.logger.Sync(); err != nil {
		fmt.Printf("Error syncing logger: %v\n", err)
	}
	
	return nil
}

func main() {
	// Create server
	server, err := NewRelayServer()
	if err != nil {
		fmt.Printf("Failed to create server: %v\n", err)
		os.Exit(1)
	}
	
	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()
	
	// Wait for signal or error
	select {
	case sig := <-sigChan:
		server.logger.Info("Received signal, initiating graceful shutdown",
			zap.String("signal", sig.String()),
		)
	case err := <-errChan:
		server.logger.Error("Server error",
			zap.Error(err),
		)
	}
	
	// Graceful shutdown
	if err := server.Stop(); err != nil {
		fmt.Printf("Error during shutdown: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Println("Server stopped gracefully")
}