package install

import (
	"fmt"
	"path/filepath"
)

// Installer manages the installation process
type Installer struct {
	config  *Config
	verbose bool
}

// New creates a new installer instance
func New(config *Config, verbose bool) *Installer {
	return &Installer{
		config:  config,
		verbose: verbose,
	}
}

// Run executes the full installation process
func (i *Installer) Run() error {
	c := i.config

	// Initialize installation log file
	logPath := filepath.Join(c.DeploboxHome, "deployments.log")

	if err := initInstallLog(logPath); err != nil {
		return fmt.Errorf("initializing log file: %w", err)
	}

	defer closeInstallLog()

	fmt.Println()
	fmt.Println("===========================================")
	fmt.Println("==   Deplobox installation starting...   ==")
	fmt.Println("== Ryan Weber Ltd https://ryan-weber.com ==")
	fmt.Println("===========================================")
	fmt.Println()

	// Install required packages
	packages := []string{"sudo", "git", "gh", "nginx", "certbot", "python3-certbot-nginx", "curl"}

	for _, pkg := range packages {
		if err := ensurePackage(pkg); err != nil {
			return fmt.Errorf("installing package %s: %w", pkg, err)
		}
	}

	// Setup system
	steps := []struct {
		name string
		fn   func(*Config) error
	}{
		{"creating user", ensureUser},
		{"setting up projects directory", setupProjectsDir},
		{"setting up SSH", setupSSH},
		{"uploading deploy key", uploadDeployKey},
		{"cloning repository", ensureGitClone},
		{"writing projects.yaml", writeProjectsYAML},
		{"installing service", installService},
		{"configuring nginx", installNginx},
		{"setting up SSL", setupCertbot},
		{"creating webhook", createWebhook},
	}

	for _, step := range steps {
		if err := step.fn(c); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}

	i.printSuccessSummary()

	return nil
}

func (i *Installer) printSuccessSummary() {
	c := i.config

	fmt.Println()
	fmt.Println("==========================================")
	fmt.Printf("  %sDeplobox Installation Complete!%s\n", colorGreen, colorReset)
	fmt.Println("==========================================")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Binary:     %s/deplobox\n", c.DeploboxHome)
	fmt.Printf("  Config:     /etc/deplobox/projects.yaml\n")
	fmt.Printf("  Logs:       %s/deployments.log\n", c.DeploboxHome)
	fmt.Printf("  Database:   %s/deployments.db\n", c.DeploboxHome)
	fmt.Printf("  Project:    %s/%s\n", c.ProjectsRoot, c.ProjectName)
	fmt.Printf("  Webhook URL: %s\n", c.WebhookURL)
	fmt.Printf("  Project URL: https://%s\n", c.ProjectDomain)
	fmt.Println()
	fmt.Println("Service Management:")
	fmt.Println("  Status:     systemctl status deplobox")
	fmt.Println("  Logs:       journalctl -u deplobox -f")
	fmt.Println("  Restart:    systemctl restart deplobox")
	fmt.Println()
	fmt.Println("Health Check:")
	fmt.Printf("  curl %s/health\n", c.WebhookURL)
	fmt.Println()
	fmt.Println("Webhook Endpoint:")
	fmt.Printf("  %s/in/%s\n", c.WebhookURL, c.ProjectName)
	fmt.Println()
	fmt.Println("Next Steps:")
	fmt.Printf("  1. Test webhook: Push to %s\n", c.OwnerRepo)
	fmt.Printf("  2. Check logs: journalctl -u deplobox -f\n")
	fmt.Printf("  3. View status: curl %s/status/%s\n", c.WebhookURL, c.ProjectName)
	fmt.Println()
}
