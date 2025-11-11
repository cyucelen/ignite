<div align="center">
  <img src="assets/ignite.png" alt="Ignite Logo" width="120"/>
  <h1>Ignite</h1>
  <p><strong>Fast, reliable API for on-demand Docker containers</strong></p>
  <p>Built for AI agents, automation workflows, and dynamic compute workloads</p>
</div>

## Why Ignite?

Fly.io and similar managed platforms are excellent for on-demand workloads, but suffer from reliability issues—containers sometimes fail to start without clear errors, and managing edge cases can be difficult. Complex orchestration solutions like Kubernetes work well but are overkill for simple on-demand container management.

**Ignite bridges this gap** with a lightweight, self-hosted API that provides:

- **Fast startup times**: Containers launch quickly with automatic image pulling and caching
- **Predictable reliability**: Robust error handling with specific codes for all failure modes
- **Full control**: Customize container lifecycle, resource limits, and cleanup behavior
- **Zero complexity**: Simple REST API—no cluster management, no configuration overhead
- **Built-in Web UI**: Real-time monitoring dashboard with live metrics, logs, and container management

## Perfect For

### AI Agent Workloads

Run containerized AI agents on-demand for tasks like:

- **Meeting bots**: Spin up agents to join video calls, take notes, and transcribe conversations
- **Web scrapers**: Launch browser automation agents with Playwright/Selenium in isolated containers
- **Code executors**: Run untrusted user code safely in sandboxed environments
- **Data processors**: Process large datasets with temporary compute instances that auto-cleanup

**Example**: Your AI assistant needs to join a Zoom meeting. With Ignite, you can:
```bash
# Provision a meeting bot container instantly
curl -X POST http://localhost:3000/api/containers \
  -H "Content-Type: application/json" \
  -d '{
    "image": "meeting-bot:latest",
    "command": ["python", "join_meeting.py"],
    "environment": {
      "ZOOM_MEETING_ID": "123-456-789",
      "BOT_NAME": "AI Note Taker"
    }
  }'
```

The container automatically cleans up when the meeting ends—no manual intervention required.

## Key Features

- **Lightning Fast**: Start containers quickly with automatic image pulling and caching. No pre-pulling required—just specify any Docker image and go.
- **Rock Solid**: Intelligent error handling with specific codes for image pull failures (404, 401, 429, 503, 504, 507). Clear diagnostics for all failure modes.
- **Zero Maintenance**: Containers automatically clean up on process exit. No resource leaks, no manual cleanup scripts, no orphaned containers.
- **Built-in Observability**: Real-time WebSocket updates for container lifecycle events. Web UI with live metrics, logs, and status tracking.

## Quick Start

### Prerequisites

- Docker installed and running
- Go 1.21+ (for building from source)

### Installation

```bash
# Clone and build
git clone https://github.com/cyucelen/ignite.git
cd ignite
go build -o ignite cmd/api/main.go

# Run
PORT=3000 MAX_CONTAINERS=50 ./ignite
```

### Docker Installation

```bash
# Build image
docker build -t ignite:latest .

# Run with Docker
docker run -d \
  -p 3000:3000 \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e MAX_CONTAINERS=50 \
  ignite:latest
```

### First Container

```bash
# Create a container
curl -X POST http://localhost:3000/api/containers \
  -H "Content-Type: application/json" \
  -d '{
    "image": "alpine:latest",
    "command": ["echo", "Hello from Ignite!"],
    "environment": {
      "MY_VAR": "my_value"
    }
  }'

# Response (201 Created)
{
  "id": "abc123",
  "image": "alpine:latest",
  "state": "creating",
  "created_at": "2025-11-11T12:00:00Z"
}

# Check status
curl http://localhost:3000/api/containers/abc123

# View all containers
curl http://localhost:3000/api/containers

# Stop and remove
curl -X DELETE http://localhost:3000/api/containers/abc123
```

## API Reference

### Create Container

**POST** `/api/containers`

```json
{
  "image": "python:3.11-slim",
  "command": ["python", "script.py"],
  "environment": {
    "API_KEY": "secret",
    "DEBUG": "true"
  },
  "registryAuth": {
    "username": "myuser",
    "password": "mypass"
  }
}
```

**Response Codes:**
- `201`: Container created successfully
- `400`: Invalid request (missing image, malformed JSON)
- `401`: Private image requires authentication
- `404`: Image not found in registry
- `429`: Container limit reached or registry rate limit
- `503`: Registry unreachable
- `504`: Image pull timeout
- `507`: Insufficient disk space

### List Containers

**GET** `/api/containers`

Returns all active containers with current state, uptime, and resource usage.

### Get Container Status

**GET** `/api/containers/{id}`

```json
{
  "id": "abc123",
  "image": "python:3.11-slim",
  "state": "running",
  "created_at": "2025-11-11T12:00:00Z",
  "started_at": "2025-11-11T12:00:05Z",
  "exit_code": null,
  "metrics": {
    "cpu_percent": 12.5,
    "memory_usage_mb": 128,
    "memory_limit_mb": 512,
    "uptime_seconds": 42
  }
}
```

### Stop Container

**DELETE** `/api/containers/{id}`

Gracefully stops and removes the container. Sends SIGTERM, waits for clean shutdown, then force-kills if needed.

### Health Check

**GET** `/api/health`

```json
{
  "status": "healthy",
  "docker_connected": true,
  "active_containers": 3,
  "max_containers": 50
}
```

### WebSocket Events

**GET** `/api/ws/containers`

Real-time container events:
- `container.created`
- `container.started`
- `container.exited`
- `container.removed`
- `metrics.update` (CPU, memory, network)

### Private Registry Images

Ignite supports pulling images from **GitHub Container Registry (GHCR)** for private repositories. Support for additional registries (Docker Hub, AWS ECR, Google GCR) is planned for future releases.

#### Authentication

To use private images from GHCR, provide registry credentials in the `registryAuth` field:

```bash
curl -X POST http://localhost:3000/api/containers \
  -H "Content-Type: application/json" \
  -d '{
    "image": "ghcr.io/your-org/private-image:latest",
    "command": ["python", "app.py"],
    "registryAuth": {
      "username": "your-github-username",
      "password": "ghp_your_personal_access_token"
    }
  }'
```

#### Creating a GitHub Personal Access Token

1. Go to GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Click "Generate new token (classic)"
3. Select scopes: `read:packages` (required for pulling private images)
4. Copy the generated token (starts with `ghp_`)
5. Use your GitHub username and the token in `registryAuth`

**Note**: The `password` field accepts GitHub Personal Access Tokens, not your GitHub account password.

#### Error Codes

- **401**: Authentication failed (invalid username or token)
- **404**: Image not found (check repository name and permissions)
- **429**: Rate limit exceeded (authenticate to increase limit)

## Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | HTTP server port |
| `MAX_CONTAINERS` | `50` | Maximum concurrent containers |
| `DOCKER_HOST` | `unix:///var/run/docker.sock` | Docker daemon socket |
| `CONTAINER_TIMEOUT` | `300` | Seconds before force-stop |
| `IMAGE_PULL_TIMEOUT` | `5m` | Max image pull time (2m-15m) |
| `SHUTDOWN_TIMEOUT` | `30` | Graceful shutdown timeout (seconds) |

## Use Case Examples

### AI Meeting Bot

```python
import requests

# Start meeting bot when scheduled event begins
response = requests.post('http://localhost:3000/api/containers', json={
    'image': 'ghcr.io/company/meeting-bot:latest',
    'command': ['python', 'bot.py', '--meeting-id', meeting_id],
    'environment': {
        'ZOOM_API_KEY': os.getenv('ZOOM_API_KEY'),
        'OPENAI_API_KEY': os.getenv('OPENAI_API_KEY'),
        'MEETING_DURATION': '60',
    },
    'registryAuth': {
        'username': os.getenv('GHCR_USER'),
        'password': os.getenv('GHCR_TOKEN')
    }
})

container_id = response.json()['id']

# Bot automatically exits when meeting ends
# Container auto-cleans up on exit
```

### Automated Code Execution

```javascript
// Execute user-submitted code in isolated container
const result = await fetch('http://localhost:3000/api/containers', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    image: 'node:18-alpine',
    command: ['node', '-e', userCode],
    environment: {
      EXECUTION_TIMEOUT: '30',
      MEMORY_LIMIT: '256m'
    }
  })
});

const { id } = await result.json();

// Poll for completion or subscribe to WebSocket for updates
```

## Web UI

Access the web dashboard at `http://localhost:3000` to:

- View all containers with real-time status updates
- Monitor CPU, memory, and network usage
- Stream container logs
- Start, stop, and remove containers
- Track lifecycle events and state transitions

The UI connects via WebSocket for instant updates without polling.

<div align="center">
  <img src="assets/webui.png" alt="Ignite Web UI" width="800"/>
</div>

## Comparison with Alternatives

| Feature | Ignite | Fly.io | AWS Fargate | Kubernetes |
|---------|--------|--------|-------------|------------|
| **Setup Complexity** | **Minimal** | Low | Medium | High |
| **Self-hosted** | ✅ | ❌ | ❌ | ✅ |
| **Cost** | **Free** | $$$$ | $$$ | Free* |
| **Learning Curve** | **Simple** | Medium | Medium | Steep |
| **Best For** | Simple on-demand | Global edge | AWS ecosystem | Large-scale orchestration |

*Kubernetes is free but requires significant infrastructure and operational overhead

Ignite excels at **simple, ephemeral workloads** where you need on-demand containers without the complexity of full orchestration platforms.

## Development

```bash
# Run tests
go test ./...

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Lint code
golangci-lint run

# Build for production
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ignite cmd/api/main.go
```

## Troubleshooting

### Container fails to start

Check Docker daemon:
```bash
docker ps  # Verify Docker is running
curl http://localhost:3000/api/health  # Check API health
```

### Image pull errors

- **404**: Image name/tag incorrect
- **401**: Private image needs `registryAuth`
- **429**: Registry rate limit—wait or authenticate
- **507**: Free up disk space: `docker system prune`

### Container limit reached

Increase `MAX_CONTAINERS` or wait for existing containers to finish.

### Permission denied on Docker socket

Ensure Ignite has access to `/var/run/docker.sock`:
```bash
# Add user to docker group
sudo usermod -aG docker $USER

# Or run with elevated permissions (not recommended for production)
```

## Contributing

Contributions welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure `go test ./...` passes
5. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details

