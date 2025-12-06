# Deplobox (Go)

A secure, lightweight GitHub webhook receiver for automated deployments. Written in Go for zero-dependency deployment.

## Features

- ✅ GitHub webhook signature verification (HMAC-SHA256)
- ✅ Automated git pull on push events
- ✅ Sequential post-deploy command execution
- ✅ Per-project deployment locking (prevents concurrent deployments)
- ✅ SQLite deployment history tracking
- ✅ Rate limiting (12/min global; 4/min per webhook)
- ✅ Health and status endpoints
- ✅ Comprehensive configuration validation
- ✅ Security: no shell injection, output hiding, path validation

## Quick Start

### 1. Build from Source

```bash
# Clone repository
git clone https://github.com/user/deplobox
cd deplobox

# Build
make build

# Or build manually
go build -o deplobox cmd/deplobox/main.go
```

### 2. Cross-Compile for Linux (from macOS)

```bash
make cross-compile

# Produces:
# - deplobox-linux-amd64 (Intel/AMD servers)
# - deplobox-linux-arm64 (ARM servers)
```

### 3. Configure Projects

Create `projects.yaml`:

```yaml
projects:
  my-website:
    path: /var/www/my-website
    secret: your-webhook-secret-min-32-chars-long
    branch: main
    pull_timeout: 60
    post_deploy_timeout: 300
    post_deploy:
      - npm install --production
      - npm run build
      - systemctl restart my-website
```

### 4. Deploy to Server

```bash
# Copy binary
scp deplobox-linux-amd64 user@server:/home/deploybot/deplobox

# Copy config
scp projects.yaml user@server:/home/deploybot/

# Set permissions
ssh user@server "chmod +x /home/deploybot/deplobox"
```

### 5. Set Up systemd Service

```bash
# Copy service file
sudo cp stubs/deplobox.service /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Start service
sudo systemctl start deplobox
sudo systemctl enable deplobox

# Check status
sudo systemctl status deplobox
```

## Usage

### Running Manually

```bash
# With default paths
./deplobox

# With custom config
./deplobox -config /path/to/projects.yaml

# With custom log and database paths
./deplobox -log /var/log/deplobox.log -db /var/lib/deplobox/deployments.db

# Different port
./deplobox -port 8080
```

### Environment Variables

- `DEPLOBOX_CONFIG_FILE` - Path to projects.yaml (default: ./projects.yaml)
- `DEPLOBOX_LOG_FILE` - Log file path (default: ./deployments.log)
- `DEPLOBOX_DB_PATH` - SQLite database path (default: ./deployments.db)
- `DEPLOBOX_HOST` - HTTP host (default: 127.0.0.1)
- `DEPLOBOX_PORT` - HTTP port (default: 5000)
- `DEPLOBOX_SKIP_VALIDATION` - Skip config validation (testing only)
- `DEPLOBOX_EXPOSE_OUTPUT` - Include command output in responses (insecure!)
- `DEPLOBOX_PROJECTS_ROOT` - Optional path restriction

### API Endpoints

**POST /in/{project}** - GitHub webhook endpoint

```bash
curl -X POST http://localhost:5000/in/my-website \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-Hub-Signature-256: sha256=..." \
  -d '{"ref":"refs/heads/main","after":"abc123..."}'
```

**GET /health** - Health check

```bash
curl http://localhost:5000/health
# {"status":"ok","projects":["my-website"],"project_count":1}
```

**GET /status/{project}** - Deployment history

```bash
curl http://localhost:5000/status/my-website
# {"project":"my-website","latest_deployment":{...},"recent_deployments":[...]}
```

## Configuration

### Project Configuration

```yaml
projects:
  project-name:
    # Required fields
    path: /absolute/path/to/project # Must exist, contain .git
    secret: min-32-char-webhook-secret # HMAC signature key

    # Optional fields
    branch: main # Default: main
    pull_timeout: 60 # Default: 60 seconds
    post_deploy_timeout: 300 # Default: 300 seconds
    post_deploy: # Default: []
      - command arg1 arg2 # String format (shell-quoted)
      - ["command", "arg1", "arg2"] # List format (preferred)
```

### Validation Rules

- **Path**: Must be absolute, exist, contain `.git`, optionally within `DEPLOBOX_PROJECTS_ROOT`
- **Secret**: Minimum 32 characters, no placeholder values
- **Timeouts**: Must be positive integers
- **Branch**: Non-empty string, cannot start with `-`
- **Post-deploy**: List of strings or lists (executed sequentially)

## Development

### Building

```bash
make build          # Build for current platform
make cross-compile  # Build for Linux (AMD64 + ARM64)
```

### Testing

```bash
make test           # Run all tests
make test-coverage  # Generate coverage report
```

### Linting & Formatting

```bash
make lint           # Run go vet
make fmt            # Format code with gofmt
```

## Makefile Targets

```bash
make build          # Build the binary
make test           # Run tests
make test-coverage  # Run tests with HTML coverage report
make clean          # Clean build artifacts
make install        # Install to /usr/local/bin
make uninstall      # Remove from /usr/local/bin
make run            # Run with default config
make cross-compile  # Build for Linux (AMD64 + ARM64)
make lint           # Run linter
make fmt            # Format code
make deps           # Update dependencies
make help           # Show all targets
```

## Security

### Signature Verification

All webhooks must have valid HMAC-SHA256 signatures in `X-Hub-Signature-256` header.

### Input Validation

- Content-Type must be `application/json`
- `X-GitHub-Event` must be `push`
- Payload size capped at 1 MB
- Branch names validated (no leading `-`)

### Command Execution

- Uses `exec.Command` without shell (no shell injection)
- Commands parsed with proper quoting (go-shellquote)
- Timeouts prevent hanging processes
- Output sanitized (hidden by default)

### Configuration

- Paths validated (absolute, exist, contain `.git`)
- Secrets minimum 32 characters
- Optional `DEPLOBOX_PROJECTS_ROOT` restriction

### Concurrency

- Per-project mutexes prevent concurrent git operations
- Returns 429 if deployment already in progress

## Troubleshooting

### Binary won't run on Linux

Make sure you built for the correct architecture:

```bash
# Check server architecture
uname -m

# x86_64 → Use deplobox-linux-amd64
# aarch64 → Use deplobox-linux-arm64
```

### Permission denied

```bash
chmod +x deplobox
```

### Cannot connect to server

Check host/port settings:

```bash
./deplobox -host 0.0.0.0 -port 5000
```

For production, use nginx reverse proxy (see Python version docs).

### Webhook failing with 403

Check signature secret matches GitHub webhook configuration.

### Logs not appearing

Check log file permissions:

```bash
touch deployments.log
chmod 644 deployments.log
```

## License

MIT

## Credits

Author: Ryan Weber Ltd - https://ryan-weber.com
