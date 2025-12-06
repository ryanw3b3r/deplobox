# Deplobox Installer

A single-binary Go installer for deplobox that automates the entire server setup process.

## Features

- **Single static binary** - Just copy one file to your server
- **Config file support** - Pre-configure answers, only prompt for missing values
- **Two-domain setup** - Separate webhook URL and project domain
- **Automated GitHub integration** - Auto-creates deploy keys and webhooks
- **Interactive prompts** - Guides you through setup if no config provided
- **Idempotent** - Safe to run multiple times

## Quick Start

### 1. Build the installer

```bash
cd installer
make build-linux
```

This creates `deplobox-installer-linux-amd64` - a single binary you can copy to your server.

### 2. Copy to server

```bash
scp deplobox-installer-linux-amd64 user@server:/tmp/deplobox-installer
```

### 3. Run on server

```bash
# Interactive mode (prompts for everything)
sudo /tmp/deplobox-installer

# With config file
sudo /tmp/deplobox-installer --config config.yaml

# Command line flags
sudo /tmp/deplobox-installer \
  --webhook-url https://server.example.com \
  --owner-repo username/my-app \
  --project-domain myapp.example.com
```

## Configuration

### Config File

Create `installer/config.yaml` with your values:

```yaml
# Reusable settings (same across projects)
webhook_url: "https://server.example.com"
certbot_email: "tech@example.com"
deploy_user: "deploybot"
deploy_group: "www-data"
projects_root: "/var/www/projects"
deplobox_home: "/home/deploybot/deplobox"
github_token: "ghp_xxxxxxxxxxxx"

# Project-specific
owner_repo: "username/my-app"
project_name: "my-app"
project_domain: "myapp.example.com"
```

See `installer-config.example.yaml` for full documentation.

### Two Domain System

The installer uses two different domains:

1. **Webhook URL** (`webhook_url`)

   - Where deplobox service is hosted
   - Receives GitHub webhooks
   - Reusable across all projects
   - Example: `https://server.example.com`
   - Webhook endpoint: `https://server.example.com/in/my-app`

2. **Project Domain** (`project_domain`)
   - Where the actual project/app is hosted
   - Different for each project
   - Example: `myapp.example.com`

This separation allows you to:

- Host deplobox on one server/domain
- Deploy multiple projects to different domains
- Reuse the same webhook service for multiple repos

### Config Priority

Values are loaded in this order (later overrides earlier):

1. Default values
2. Config file (`--config` or auto-detected)
3. Command line flags
4. Environment variables (`GH_TOKEN`, `GITHUB_TOKEN`)
5. Interactive prompts (only for missing required values)

### Auto-detected Config Locations

If you don't specify `--config`, the installer checks:

1. `./installer/config.yaml` (current directory)
2. `/etc/deplobox/installer-config.yaml` (system-wide)

## What It Does

The installer automates the entire deplobox setup:

1. **Package Installation**

   - git, nginx, certbot, gh CLI, curl

2. **User Setup**

   - Creates deploy user
   - Adds to web server group

3. **Directory Structure**

   - Projects root
   - Deplobox home
   - Config directory

4. **SSH Configuration**

   - Generates ED25519 deploy key
   - Configures SSH host alias
   - Sets proper permissions

5. **GitHub Integration** (if token provided)

   - Uploads deploy key to repository
   - Creates webhook with secret

6. **Repository Clone**

   - Clones project via SSH
   - Sets ownership

7. **Deplobox Deployment**

   - Copies/downloads binary
   - Creates projects.yaml config

8. **System Services**
   - Creates systemd service
   - Configures nginx reverse proxy
   - Sets up Let's Encrypt SSL (if email provided)
   - Starts deplobox service

## Command Line Options

```
--config <path>          Config file path
--webhook-url <url>      Webhook URL (where deplobox is hosted)
--project-domain <url>   Project domain (where project is hosted)
--owner-repo <repo>      GitHub owner/repo
--project-name <name>    Project slug
--certbot-email <email>  Email for Let's Encrypt
--deploy-user <user>     Deploy user (default: deploybot)
--deploy-group <group>   Web group (default: www-data)
--projects-root <path>   Projects directory
--deplobox-home <path>   Deplobox directory
--github-token <token>   GitHub token
--binary-source <src>    'local' or URL
--verbose               Verbose output
--version               Show version
--help                  Show help
```

## Examples

### Fully Automated Install

```bash
# Create config with all values
cat > installer/config.yaml <<EOF
webhook_url: "https://server.example.com"
certbot_email: "tech@example.com"
github_token: "ghp_xxxxxxxxxxxx"
owner_repo: "username/my-app"
project_domain: "myapp.example.com"
EOF

# Run installer (no prompts)
sudo deplobox-installer --config config.yaml
```

### Partial Config + Prompts

```bash
# Config has reusable settings
cat > installer/config.yaml <<EOF
webhook_url: "https://server.example.com"
certbot_email: "tech@example.com"
deploy_user: "deploybot"
EOF

# Installer will prompt for project-specific values
sudo deplobox-installer --config config.yaml
```

### Environment Variable Token

```bash
export GH_TOKEN="ghp_xxxxxxxxxxxx"
sudo -E deplobox-installer --config config.yaml
```

### Override Config Values

```bash
# Use config but override project
sudo deplobox-installer \
  --config base-config.yaml \
  --owner-repo username/different-app \
  --project-name different-app
```

## Building

```bash
# Current platform
make build

# Linux only (for deployment)
make build-linux

# Multiple platforms
make cross-compile

# Clean build artifacts
make clean

# Install dependencies
make deps
```

## GitHub Token

The installer can automate GitHub setup if you provide a Personal Access Token.

**Required scopes:**

- `repo` - Access repository
- `admin:public_key` - Manage deploy keys
- `write:repo_hook` - Create webhooks

**How to provide:**

1. Config file: `github_token: "ghp_xxx"`
2. Environment: `GH_TOKEN=ghp_xxx` or `GITHUB_TOKEN=ghp_xxx`
3. Command line: `--github-token ghp_xxx`

**If not provided:**

- Deploy key and webhook setup will be skipped
- You'll need to set these up manually in GitHub

## Differences from install.sh

The new Go installer improves on the bash version:

1. **Config file support** - Pre-configure common values
2. **Two domains** - Separate webhook URL from project domain
3. **Better error handling** - Type-safe, clear error messages
4. **Easier to test** - Can write unit tests
5. **Single binary** - No bash dependencies
6. **Idempotent** - Safe to re-run
7. **Better UX** - Colored output, progress indicators
8. **Validated input** - Type checking on config values

## Troubleshooting

### Service fails to start

```bash
# Check service status
systemctl status deplobox

# View logs
journalctl -u deplobox -n 50

# Check config
cat /home/deploybot/deplobox/projects.yaml
```

### SSH key issues

```bash
# Check deploy key permissions
ls -la /home/deploybot/.ssh/

# Test GitHub connection
sudo -u deploybot ssh -T git@github.<project>
```

### Nginx errors

```bash
# Test nginx config
nginx -t

# Check nginx logs
tail -f /var/log/nginx/error.log
```

## License

Same as deplobox main project
