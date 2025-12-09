# Deplobox

A secure, lightweight GitHub webhook receiver for automated deployments. Written in Go with zero dependencies and comprehensive security hardening.

## Features

### Deployment

- ✅ **Zero-downtime deployments** with Capistrano-style releases
- ✅ Atomic symlink switching for instant cutover
- ✅ Automated release management (keeps last 5 releases)
- ✅ Shared files support (env, storage, uploads persist across deployments)
- ✅ Sequential post-deploy command execution with timeouts
- ✅ Per-project deployment locking (prevents concurrent deployments)

### Security

- ✅ **Command injection prevention** - Git URL, branch, and project name validation
- ✅ **Path traversal protection** - Symlink and path sanitization
- ✅ **Enhanced secret validation** - 48 char minimum with Shannon entropy checking
- ✅ **Secure file permissions** - 0640 for logs/configs, 0600 for SSH keys
- ✅ GitHub webhook signature verification (HMAC-SHA256)
- ✅ Input sanitization for all user-provided data
- ✅ No shell execution - direct `exec.Command` usage
- ✅ Rate limiting (12/hour global; 4/min per webhook)

### Monitoring & Management

- ✅ SQLite deployment history tracking with full audit trail
- ✅ Health and status endpoints with deployment metrics
- ✅ Structured JSON logging with `log/slog`
- ✅ Comprehensive configuration validation
- ✅ **90%+ test coverage** for security-critical packages

## Quick Start

### 1. Build from Source

```bash
# Clone repository
git clone https://github.com/user/deplobox
cd deplobox

# Build for current platform
make build

# Build distribution packages for all platforms
make dist

# This creates:
# - dist/macos/ (macOS ARM64)
# - dist/linux-amd64/ (Intel/AMD servers)
# - dist/linux-arm64/ (ARM servers)
#
# Each folder contains:
# - deplobox (single binary with install, serve, version subcommands)
# - config/projects.example.yaml
# - config/installer.example.yaml
# - templates/nginx-site.template
# - templates/systemd-service.template
#
# Plus compressed archives:
# - dist/deplobox-macos.zip
# - dist/deplobox-linux-amd64.zip
# - dist/deplobox-linux-arm64.zip
```

### 2. Download Pre-built Release

Download the appropriate archive for your platform from the releases page:

- `deplobox-macos.zip` for macOS
- `deplobox-linux-amd64.zip` for Linux (Intel/AMD)
- `deplobox-linux-arm64.zip` for Linux (ARM)

Extract the archive:

```bash
# For macOS
unzip deplobox-macos.zip -d deplobox

# For Linux
tar -xzf deplobox-linux-amd64.zip -d deplobox
```

### 3. Configure Projects

Deplobox uses zero-downtime deployments. Each project needs this structure:

```
/var/www/projects/my-website/
├── shared/              # Persistent files (e.g., .env, storage)
├── releases/            # Timestamped releases
│   └── 2025-12-07-13-08-03/
└── current -> releases/2025-12-07-13-08-03/
```

Create `projects.yaml` (or copy from `config/projects.example.yaml`):

```yaml
projects:
  my-website:
    path: /var/www/projects/my-website # Project root (NOT current!)
    secret: your-webhook-secret-min-32-chars-long
    branch: main
    pull_timeout: 60
    post_deploy_timeout: 300
    post_deploy:
      - npm install --production
      - npm run build
    post_activate_timeout: 300
    post_activate:
      - pm2 reload my-website
```

**Note**: The `path` must point to the project root containing `shared/`, `releases/`, and `current`. The installer creates this structure automatically.

See [ZERO-DOWNTIME.md](ZERO-DOWNTIME.md) for detailed documentation.

### 4. Deploy to Server

```bash
# Copy distribution to server
scp deplobox-linux-amd64.zip user@server:/tmp/
ssh user@server

# Extract and run installer
cd /tmp
tar -xzf deplobox-linux-amd64.zip
cd deplobox

# Configure installer (edit config/installer.yaml)
nano config/installer.yaml

# Run installation (sets up nginx, systemd, GitHub webhooks)
sudo ./deplobox install --config config/installer.yaml

# Or run with prompts
sudo ./deplobox install
```

The installer will:

- Install system packages (nginx, git, certbot)
- Create deploy user and SSH keys
- Clone your repository
- Configure nginx site
- Set up systemd service
- Upload SSH deploy key to GitHub (if token provided)
- Create GitHub webhook (if token provided)
- Set up SSL with Let's Encrypt

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

### CLI Commands

Deplobox is a single binary with multiple subcommands:

```bash
# Show help
./deplobox --help

# Install and configure deplobox on a server
./deplobox install [--config installer.yaml] [--verbose]

# Start the webhook server
./deplobox serve [--config projects.yaml] [--port 5000] [--host 127.0.0.1]

# Show version information
./deplobox version
```

### Running the Server

```bash
# With default paths
./deplobox serve

# With custom config
./deplobox serve --config /path/to/projects.yaml

# With custom log and database paths
./deplobox serve --log /var/log/deplobox.log --db /var/lib/deplobox/deployments.db

# Different port and host
./deplobox serve --port 8080 --host 0.0.0.0
```

### Environment Variables

- `DEPLOBOX_CONFIG_FILE` - Path to projects.yaml
- `DEPLOBOX_LOG_FILE` - Log file path (default: ./deployments.log)
- `DEPLOBOX_DB_PATH` - SQLite database path (default: ./deployments.db)
- `DEPLOBOX_HOST` - HTTP host (default: 127.0.0.1)
- `DEPLOBOX_PORT` - HTTP port (default: 5000)
- `DEPLOBOX_SKIP_VALIDATION` - Skip config validation (testing only)
- `DEPLOBOX_EXPOSE_OUTPUT` - Include command output in responses (insecure!)
- `DEPLOBOX_PROJECTS_ROOT` - Optional path restriction

### Config File Search Paths

If no `-config` flag is provided and `DEPLOBOX_CONFIG_FILE` is not set, deplobox searches for `projects.yaml` in:

1. `./projects.yaml` (current directory)
2. `./config/projects.yaml` (config subdirectory)
3. `/etc/deplobox/projects.yaml` (system-wide)

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
    post_activate_timeout: 300 # Default: 300 seconds
    post_activate: # Default: []
      - command arg1 arg2 # Runs AFTER current symlink is updated
      - ["pm2", "reload", "app"] # Example: restart app server
```

### Validation Rules

- **Path**: Must be absolute, exist, contain `.git`, optionally within `DEPLOBOX_PROJECTS_ROOT`
- **Secret**: Minimum 32 characters, no placeholder values
- **Timeouts**: Must be positive integers
- **Branch**: Non-empty string, cannot start with `-`
- **Post-deploy**: List of strings or lists (executed sequentially, before activation)
- **Post-activate**: List of strings or lists (executed sequentially, after activation)

## Development

### Building

```bash
make build          # Build for current platform
make dist           # Build distribution packages for all platforms
make cross-compile  # Build for Linux only (legacy)
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

Deplobox implements comprehensive security hardening with **90%+ test coverage** for security-critical packages.

### Input Validation & Sanitization

- **Git URL Validation**: Only HTTPS GitHub URLs allowed, regex pattern matching
- **Project Name Validation**: Alphanumeric + dash/underscore only, no path traversal
- **Branch Name Validation**: No shell metacharacters (`;`, `|`, `&`, `` ` ``, `$`)
- **Path Traversal Protection**: Symlinks resolved with `filepath.EvalSymlinks`, canonical path checking
- **Content-Type**: Must be `application/json`
- **Event Type**: Must be `push`
- **Payload Size**: Capped at 1 MB

### Authentication & Secrets

- **Webhook Signatures**: HMAC-SHA256 verification required (GitHub `X-Hub-Signature-256` header)
- **Secret Strength**: Minimum 48 characters with Shannon entropy ≥ 3.5
- **Forbidden Values**: Rejects placeholder secrets (`topsecret`, `password`, `changeme`, `replace-with-secret`)
- **Secret Generation**: Cryptographically secure random generation with `crypto/rand`

### Command Execution

- **No Shell Execution**: Uses `exec.Command` directly, never through shell
- **Command Allowlisting**: Only specific commands allowed (git, composer, npm, php, pm2, artisan)
- **Shell Metacharacter Prevention**: Arguments validated for `;`, `|`, `&`, `$`, `` ` ``, `>`, `<`, `(`, `)`, `{`, `}`
- **Timeouts**: All commands have configurable timeouts to prevent hanging
- **Proper Quoting**: Uses `go-shellquote` for safe command parsing

### File Security

- **Secure Permissions**:
  - Config/log files: 0640 (owner + group read, owner write)
  - SSH keys: 0600 (owner only)
  - Executables: 0750 (owner execute/write, group execute)
  - Directories: 0750 (owner full, group read/execute)
- **Path Validation**: All paths must be absolute and within allowed directories
- **Symlink Safety**: Atomic symlink operations, no dangling symlinks

### Concurrency & Rate Limiting

- **Per-Project Locking**: Mutexes prevent concurrent git operations on same project
- **429 on Conflict**: Returns HTTP 429 if deployment already in progress
- **Global Rate Limit**: 12 requests per hour per IP
- **Webhook Rate Limit**: 4 requests per minute per IP
- **Token Bucket Algorithm**: Using `golang.org/x/time/rate`

### Monitoring & Audit

- **Deployment History**: Full audit trail in SQLite database
- **Structured Logging**: JSON logs with `log/slog` including request IDs, duration, status
- **Status Endpoints**: Monitor recent deployments and success rates
- **Error Recording**: Failed deployments logged with error messages

For detailed security documentation, see [SECURITY.md](SECURITY.md).

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
