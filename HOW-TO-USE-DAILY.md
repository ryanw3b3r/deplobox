# How to Use Deplobox Daily

This guide covers the everyday workflow of connecting new GitHub organizations and repositories to your deplobox server.

## Prerequisites

- Deplobox is already installed on your server (`my-server.com`)
- You have root access to the server
- You have admin access to the GitHub organization (`my-github-org`)

## Quick Story: Connecting `my-github-org/my-repo` to `my-server.com`

### Step 1: SSH into Your Server

```bash
ssh root@my-server.com
```

### Step 2: Create a GitHub Personal Access Token (if using automated setup)

If you want deplobox to automatically set up the deploy key and webhook, you need a GitHub Personal Access Token (PAT).

These keys are used per organisation, so your personal account needs a token and if you want to connect Deplobox to your organisation, you will need a separate token for this organisation too.

1. Go to https://github.com/settings/personal-access-tokens
2. Click "Generate new token" → "Generate new token"
3. Give it a name like "Deplobox - my-server.com"
4. Give it a description
5. Next is "Resource owner" so choose your personal account or organisation
6. Expiration can be anything, choose whichever you like, or "No expiration" (not recommended - you should always set expiry date)
7. Repository access is where you can give token access to only public repositories, all repositories or only selected ones. Choose as you wish.
8. Permissions - this is the most important bit. We need these scopes:
   - `Administration` - select **read and write**
   - `Contents` - select **read only**
   - `Webhooks` - select **read and write**
9. Generate the token and copy it (you won't see it again so make sure you have it!)

### Step 3: Prepare the Installer Configuration

You can use command-line flags or a config file. For repeated use, create a config file:

```bash
mkdir -p /home/deploybot/deplobox/config
nano /home/deploybot/deplobox/config/installer.yaml
```

Add the following content:

```yaml
# Reusable settings (keep these for all projects on this server)
webhook_url: 'https://my-server.com'
certbot_email: 'your-email@example.com' # For SSL certificates
deploy_user: 'deploybot'
deploy_group: 'www-data'
projects_root: '/var/www/projects'
deplobox_home: '/home/deploybot/deplobox'

# GitHub token for automated setup (optional but recommended)
github_token: 'ghp_your-token-here'

# Project-specific settings
owner_repo: 'my-github-org/my-repo'
project_name: 'my-repo'
project_domain: 'my-repo.my-server.com' # Domain where this app will be hosted
```

**Note:** If you prefer not to store the token in a file, you can set the `GH_TOKEN` environment variable instead:

```bash
export GH_TOKEN="ghp_your-token-here"
```

### Step 4: Run the Installer

```bash
cd /home/deploybot/deplobox
sudo ./deplobox install --config config/installer.yaml
```

Or without a config file (interactive mode):

```bash
sudo ./deplobox install
```

The installer will prompt you for any missing values.

### What the Installer Does Automatically

If you provided a GitHub token, the installer will:

1. **Create deploy user** - Sets up the `deploybot` user with proper permissions
2. **Generate SSH key pair** - Creates a unique deploy key for this project
3. **Upload deploy key to GitHub** - Adds the key to `my-github-org/my-repo` as a deploy key
4. **Clone the repository** - Does an initial clone to `/var/www/projects/my-repo`
5. **Create the project structure** - Sets up `shared/`, `releases/`, and `current` symlink
6. **Create/update projects.yaml** - Adds your project to `/etc/deplobox/projects.yaml`
7. **Install systemd service** - Sets up deplobox as a background service
8. **Configure nginx** - Sets up a reverse proxy for your project
9. **Create GitHub webhook** - Adds a webhook pointing to `https://my-server.com/in/my-repo`
10. **Set up SSL** - Obtains a Let's Encrypt certificate for your domain

### Step 5: Verify the Installation

```bash
# Check deplobox service status
systemctl status deplobox

# Check health endpoint
curl https://my-server.com/health

# Check project status
curl https://my-server.com/status/my-repo

# View logs
journalctl -u deplobox -f
```

### Step 6: Test the Deployment

Push a commit to the `main` branch of `my-github-org/my-repo`:

```bash
# On your local machine
cd /path/to/my-repo
echo "test deployment" >> test.txt
git add test.txt
git commit -m "Test deplobox deployment"
git push origin main
```

Then watch the logs on the server:

```bash
ssh root@my-server.com
journalctl -u deplobox -f
```

## Manual Setup (Without GitHub Token)

If you don't want to provide a GitHub token, you'll need to do some steps manually:

### Step 1: Run Installer Without Token

```bash
cd /home/deploybot/deplobox
sudo ./deplobox install
```

Fill in the values when prompted. Leave `github_token` empty.

### Step 2: Manually Add Deploy Key to GitHub

After the installer completes, it will have generated an SSH key. View the public key:

```bash
cat /home/deploybot/.ssh/my-repo.key.pub
```

1. Go to https://github.com/my-github-org/my-repo/settings/keys
2. Click "Add deploy key"
3. Paste the public key
4. Title it "my-repo-deplobox"
5. Check "Allow write access" (if you need deployment scripts to push back)
6. Click "Add deploy key"

### Step 3: Manually Create Webhook in GitHub

1. Go to https://github.com/my-github-org/my-repo/settings/hooks
2. Click "Add webhook"
3. Fill in:
   - **Payload URL**: `https://my-server.com/in/my-repo`
   - **Content type**: `application/json`
   - **Secret**: Use the secret from `/etc/deplobox/projects.yaml`
   - **Events**: Select "Just the push event"
4. Click "Add webhook"

## Adding a Second Project

To add another repository from the same or different organization:

### Option 1: Using the Same Installer Config

```bash
cd /home/deploybot/deplobox
sudo ./deplobox install \
  --owner-repo "my-github-org/another-repo" \
  --project-name "another-repo" \
  --project-domain "another-repo.my-server.com"
```

The installer will reuse the existing settings from `/etc/deplobox/installer.yaml` (if it exists) and add the new project to `/etc/deplobox/projects.yaml`.

### Option 2: Create a New Config File

```bash
nano /home/deploybot/deplobox/config/another-repo.yaml
```

```yaml
webhook_url: 'https://my-server.com'
certbot_email: 'your-email@example.com'
github_token: 'ghp_your-token-here'

owner_repo: 'my-github-org/another-repo'
project_name: 'another-repo'
project_domain: 'another-repo.my-server.com'
```

Then run:

```bash
sudo ./deplobox install --config config/another-repo.yaml
```

## Editing Project Configuration

After installation, you can edit the project configuration at any time:

```bash
nano /etc/deplobox/projects.yaml
```

Example project configuration:

```yaml
projects:
  my-repo:
    path: /var/www/projects/my-repo
    secret: your-generated-webhook-secret-min-32-chars
    branch: main
    pull_timeout: 60
    post_deploy_timeout: 300
    post_deploy:
      - npm ci --production
      - npm run build
    post_activate_timeout: 300
    post_activate:
      - pm2 reload my-repo
```

After editing, restart the service:

```bash
systemctl restart deplobox
```

## Common Post-Deploy Commands by Framework

### Node.js

```yaml
post_deploy:
  - npm ci --production
  - npm run build
post_activate:
  - pm2 reload my-app
```

### PHP / Laravel

```yaml
post_deploy:
  - composer install --no-dev --optimize-autoloader
  - php artisan migrate --force
post_activate:
  - php artisan config:cache
  - php artisan route:cache
  - php artisan queue:restart
```

### Python / Django

```yaml
post_deploy:
  - pip install -r requirements.txt
  - python manage.py collectstatic --noinput
  - python manage.py migrate --noinput
post_activate:
  - systemctl reload gunicorn
```

### Static Site

```yaml
post_deploy:
  - npm ci
  - npm run build
post_activate: []
```

## Shared Files Pattern

Persistent files (like `.env`, uploads, etc.) should be stored in the `shared/` directory:

```bash
# Create shared directories
mkdir -p /var/www/projects/my-repo/shared/.env
mkdir -p /var/www/projects/my-repo/shared/uploads

# Create symlinks in your post_deploy commands
post_deploy:
  - ln -nfs /var/www/projects/my-repo/shared/.env .env
  - ln -nfs /var/www/projects/my-repo/shared/uploads public/uploads
```

## Troubleshooting

### Webhook Not Triggering

1. Check the webhook URL in GitHub settings matches your server URL
2. Verify the secret in GitHub webhook matches `/etc/deplobox/projects.yaml`
3. Check deplobox service is running: `systemctl status deplobox`

### Permission Denied on Git Clone

1. Verify the deploy key was added to GitHub
2. Check SSH key permissions: `ls -la /home/deploybot/.ssh/`
3. Test SSH connection: `sudo -u deploybot ssh -T git@github.com`

### SSL Certificate Issues

```bash
# Manually obtain SSL certificate
sudo certbot --nginx -d my-repo.my-server.com
```

### View Deployment History

```bash
# Via API
curl https://my-server.com/status/my-repo

# Via SQLite
sqlite3 /home/deploybot/deplobox/deployments.db "SELECT * FROM deployments ORDER BY created_at DESC LIMIT 10;"
```

## Useful Commands

```bash
# Service management
systemctl status deplobox    # Check service status
systemctl restart deplobox   # Restart service
systemctl stop deplobox      # Stop service

# Logs
journalctl -u deplobox -f    # Follow logs
journalctl -u deplobox -n 100 # Last 100 lines

# Health checks
curl https://my-server.com/health
curl https://my-server.com/status/my-repo

# Restore previous release
cd /home/deploybot/deplobox
sudo ./deplobox restore my-repo
```
