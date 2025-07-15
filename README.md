# STAG Relay

A high-performance WebSocket relay service for streaming spatial data from AR/VR applications to the STAG (Spatial Tracking and Analytics Gateway) service. The relay processes StreamKit packets, transforms them into spatial events, and efficiently batches them for delivery to STAG.

## Features

- **Real-time WebSocket Communication**: Handles multiple concurrent AR/VR client connections
- **Data Processing Pipeline**: Parses, transforms, and batches spatial data packets
- **Efficient Batching**: Configurable batch sizes and timeouts for optimal performance
- **Metrics & Monitoring**: Prometheus metrics integration for observability
- **Health Checks**: Built-in health and status endpoints
- **Graceful Shutdown**: Proper resource cleanup and connection handling
- **Docker Support**: Containerized deployment with Docker Compose
- **Configuration Management**: Flexible YAML configuration with environment variable overrides

## Architecture

The relay consists of several key components:

- **Gate** (`internal/gate/`): Manages WebSocket connections and message routing
- **Parser** (`internal/parser/`): Parses incoming StreamKit packets
- **Transformer** (`internal/transformer/`): Converts parsed packets to spatial events
- **Updater** (`internal/updater/`): Batches and sends events to STAG
- **Metrics** (`internal/metrics/`): Prometheus metrics collection
- **Types** (`pkg/types/`): Shared data structures and configuration

## Data Flow

1. AR/VR clients connect via WebSocket to `/ws/streamkit`
2. StreamKit packets (pose/mesh data) are received and parsed
3. Packets are transformed into spatial events with anchors and mesh diffs
4. Events are batched and sent to the STAG service
5. Metrics are collected throughout the pipeline

## Getting Started

### Prerequisites

- Go 1.22 or higher
- Docker and Docker Compose (for containerized deployment)
- golangci-lint (for linting)

### Installation

#### Option 1: Local Development

```bash
# Clone the repository
git clone <repository-url>
cd packages/relays

# Install dependencies
make deps

# Build the application
make build

# Run the relay
./relay
```

#### Option 2: Docker Deployment

```bash
# Build and start all services
make docker-up

# View logs
make docker-logs

# Stop services
make docker-down
```

### Configuration

The relay can be configured via:

1. **Configuration file** (`config.yaml`):
```yaml
server:
  port: "8081"
  host: "0.0.0.0"

stag:
  url: "http://localhost:8080"
  timeout: "10s"

websocket:
  buffer_size: 1024
  heartbeat_interval: "30s"

batch:
  max_size: 5
  timeout: "100ms"
```

2. **Environment variables** (prefixed with `RELAY_`):
```bash
export RELAY_SERVER_PORT=8080
export RELAY_STAG_URL=http://stag-service:8080
export RELAY_BATCH_MAX_SIZE=10
```

### API Endpoints

- `GET /health` - Health check endpoint
- `GET /status` - Service status with active connections
- `GET /metrics` - Prometheus metrics
- `GET /ws/streamkit` - WebSocket endpoint for StreamKit clients

### WebSocket Protocol

Clients connect to `/ws/streamkit` and send JSON packets:

```json
{
  "session_id": "session-123",
  "frame_number": 1,
  "timestamp": 1634567890123,
  "type": "pose",
  "data": {
    "pose": {
      "x": 1.0,
      "y": 2.0,
      "z": 3.0,
      "rotation": [0.0, 0.0, 0.0, 1.0]
    }
  }
}
```

For mesh data:
```json
{
  "session_id": "session-123",
  "frame_number": 2,
  "timestamp": 1634567890124,
  "type": "mesh",
  "data": {
    "mesh": {
      "vertices": "base64-encoded-draco-data",
      "faces": "base64-encoded-draco-data",
      "anchor_id": "anchor-456"
    }
  }
}
```

## Testing

### Unit Tests

```bash
# Run all unit tests
make test-unit

# Run with coverage
make test-coverage

# Run with race detection
make test-race
```

### Integration Tests

```bash
# Run integration tests
make test-integration

# Run all tests
make test
```

### Test Structure

- `tests/unit/` - Unit tests for individual components
- `tests/integration/` - End-to-end integration tests
- `tests/benchmark/` - Performance benchmarks
- `tests/testdata/` - Test utilities and data generators

## Development

### Building

```bash
# Build the binary
make build

# Clean build artifacts
make clean

# Development build with tests
make dev
```

### Code Quality

```bash
# Run linter
make lint

# Format code
go fmt ./...

# Run tests with coverage
make test-coverage
```

### Docker Development

```bash
# Build Docker image
make docker-build

# Start development environment
make docker-up

# View logs
docker-compose -f docker/docker-compose.yml logs -f relay

# Stop environment
make docker-down
```

## Monitoring

### Prometheus Metrics

The relay exposes metrics at `/metrics`:

- `relay_packets_total` - Total packets processed by type and status
- `relay_connections_active` - Number of active WebSocket connections
- `relay_batch_size` - Batch sizes sent to STAG
- `relay_processing_duration_seconds` - Processing time per packet

### Health Checks

- `GET /health` returns service health status
- `GET /status` returns detailed service information
- Docker health check configured for container orchestration

## Production Deployment

### Docker Compose

The included `docker-compose.yml` provides:

- **Relay service** with health checks (runs independently)
- **Prometheus** for metrics collection
- **Networking** configured for service communication

**Note**: The relay service runs independently and does not require STAG to be running. It will gracefully handle STAG being unavailable and attempt to reconnect when STAG becomes available.

### Configuration

For production deployment:

1. Update `config.yaml` with production STAG URL (default: `http://host.docker.internal:8080`)
2. Set appropriate batch sizes and timeouts
3. Configure proper logging levels
4. Set up monitoring and alerting on metrics
5. Configure load balancing for multiple instances

**STAG URL Configuration**: By default, the relay connects to STAG at `http://host.docker.internal:8080` when running in Docker, allowing it to reach STAG running on the host machine. For production, update this to your actual STAG service URL.

### Environment Variables

Key production environment variables:

```bash
RELAY_STAG_URL=http://host.docker.internal:8080  # Default for Docker
RELAY_BATCH_MAX_SIZE=50
RELAY_BATCH_TIMEOUT=500ms
RELAY_WEBSOCKET_BUFFER_SIZE=4096
```

## Performance Considerations

- **Batch Size**: Larger batches reduce HTTP overhead but increase latency
- **Batch Timeout**: Lower timeouts reduce latency but increase request frequency
- **Buffer Size**: Larger buffers handle traffic spikes but use more memory
- **Connection Limits**: Monitor active connections and implement rate limiting if needed

## Troubleshooting

### Common Issues

1. **Connection Refused to STAG**
   - Check STAG service is running
   - Verify STAG URL configuration
   - Check network connectivity

2. **WebSocket Connection Issues**
   - Verify port 8081 is accessible
   - Check firewall settings
   - Validate WebSocket upgrade headers

3. **High Memory Usage**
   - Reduce buffer sizes in configuration
   - Check for connection leaks
   - Monitor batch processing efficiency

### Debugging

1. **Enable Debug Logging**:
   ```bash
   export GIN_MODE=debug
   ```

2. **Check Metrics**:
   ```bash
   curl http://localhost:8081/metrics
   ```

3. **View Health Status**:
   ```bash
   curl http://localhost:8081/health
   ```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Write tests for new functionality
4. Ensure all tests pass: `make test`
5. Run linter: `make lint`
6. Submit a pull request

## License

See [LICENSE](LICENSE) file for details.