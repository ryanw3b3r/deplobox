package main

import (
	"flag"
	"fmt"
	"os"
)

const version = "1.0.0"

func main() {
	// Parse command line flags
	configFile := flag.String("config", "", "Path to config file (YAML)")
	showVersion := flag.Bool("version", false, "Show version and exit")
	verbose := flag.Bool("verbose", false, "Show verbose output")
	showHelp := flag.Bool("help", false, "Show help message")

	// Individual config flags
	webhookURL := flag.String("webhook-url", "", "Webhook URL (domain where deplobox receives webhooks)")
	certbotEmail := flag.String("certbot-email", "", "Email for Let's Encrypt")
	deployUser := flag.String("deploy-user", "", "Unix user for deployments")
	deployGroup := flag.String("deploy-group", "", "Web group for shared permissions")
	projectsRoot := flag.String("projects-root", "", "Path to checkout projects")
	deploboxHome := flag.String("deplobox-home", "", "Directory for deplobox binary/config")
	githubToken := flag.String("github-token", "", "GitHub Personal Access Token")
	binarySource := flag.String("binary-source", "", "Binary source: 'local' or URL")
	projectName := flag.String("project-name", "", "Project slug")
	ownerRepo := flag.String("owner-repo", "", "GitHub owner/repo")
	projectDomain := flag.String("project-domain", "", "Project domain (where project is hosted)")
	webhookSecret := flag.String("webhook-secret", "", "Webhook secret")
	gitHostAlias := flag.String("git-host-alias", "", "SSH host alias")
	deployKeyFile := flag.String("deploy-key-file", "", "Deploy key filename")

	flag.Parse()

	if *showVersion {
		fmt.Printf("Deplobox Installer v%s\n", version)
		os.Exit(0)
	}

	if *showHelp {
		printHelp()
		os.Exit(0)
	}

	// Check for root
	if os.Geteuid() != 0 {
		fmt.Fprintf(os.Stderr, "Error: This installer must be run as root\n")
		fmt.Fprintf(os.Stderr, "Please run with: sudo %s\n", os.Args[0])
		os.Exit(1)
	}

	// Create config with defaults
	config := NewConfig()
	config.Verbose = *verbose

	// Load from config file if provided
	if *configFile != "" {
		fmt.Printf("Loading configuration from: %s\n", *configFile)
		if err := config.LoadFromFile(*configFile); err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config file: %v\n", err)
			os.Exit(1)
		}
	}

	// Try default config file locations if no explicit config provided
	if *configFile == "" {
		for _, path := range []string{
			"./installer/config.yaml",
			"/etc/deplobox/installer-config.yaml",
		} {
			if _, err := os.Stat(path); err == nil {
				fmt.Printf("Found config file: %s\n", path)
				if err := config.LoadFromFile(path); err != nil {
					fmt.Fprintf(os.Stderr, "Error loading config file: %v\n", err)
					os.Exit(1)
				}
				break
			}
		}
	}

	// Override with command line flags
	flags := map[string]string{
		"webhook-url":     *webhookURL,
		"certbot-email":   *certbotEmail,
		"deploy-user":     *deployUser,
		"deploy-group":    *deployGroup,
		"projects-root":   *projectsRoot,
		"deplobox-home":   *deploboxHome,
		"github-token":    *githubToken,
		"binary-source":   *binarySource,
		"project-name":    *projectName,
		"owner-repo":      *ownerRepo,
		"project-domain":  *projectDomain,
		"webhook-secret":  *webhookSecret,
		"git-host-alias":  *gitHostAlias,
		"deploy-key-file": *deployKeyFile,
	}
	config.SetFromFlags(flags)

	// Load from environment
	config.LoadFromEnv()

	// Prompt for missing values
	if err := PromptForMissingValues(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error during prompts: %v\n", err)
		os.Exit(1)
	}

	// Fill derived values
	config.FillDerivedValues()

	// Validate configuration
	if err := config.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Print configuration summary
	printConfigSummary(config)

	// Confirm before proceeding
	if isInteractive() {
		fmt.Println()
		fmt.Printf("Proceed with installation? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Println("Installation cancelled")
			os.Exit(0)
		}
	}

	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("  Deplobox Installation Starting...")
	fmt.Println("==========================================")
	fmt.Println()

	// Run installation steps
	if err := runInstallation(config); err != nil {
		fmt.Fprintf(os.Stderr, "\n%sInstallation failed: %v%s\n", colorRed, err, colorReset)
		os.Exit(1)
	}

	// Print success summary
	printSuccessSummary(config)
}

func runInstallation(c *Config) error {
	// Install required packages
	packages := []string{"sudo", "git", "gh", "nginx", "certbot", "python3-certbot-nginx", "curl"}
	for _, pkg := range packages {
		if err := ensurePackage(pkg); err != nil {
			return fmt.Errorf("installing package %s: %w", pkg, err)
		}
	}

	// Setup system
	if err := ensureUser(c); err != nil {
		return fmt.Errorf("creating user: %w", err)
	}

	if err := setupProjectsDir(c); err != nil {
		return fmt.Errorf("setting up projects directory: %w", err)
	}

	if err := setupDeploboxHome(c); err != nil {
		return fmt.Errorf("setting up deplobox home: %w", err)
	}

	if err := setupSSH(c); err != nil {
		return fmt.Errorf("setting up SSH: %w", err)
	}

	// GitHub integration
	if err := uploadDeployKey(c); err != nil {
		return fmt.Errorf("uploading deploy key: %w", err)
	}

	// Clone repository
	if err := ensureGitClone(c); err != nil {
		return fmt.Errorf("cloning repository: %w", err)
	}

	// Deploy deplobox
	if err := writeProjectsYAML(c); err != nil {
		return fmt.Errorf("writing projects.yaml: %w", err)
	}

	if err := deployBinary(c); err != nil {
		return fmt.Errorf("deploying binary: %w", err)
	}

	// System services
	if err := installService(c); err != nil {
		return fmt.Errorf("installing service: %w", err)
	}

	if err := installNginx(c); err != nil {
		return fmt.Errorf("configuring nginx: %w", err)
	}

	if err := setupCertbot(c); err != nil {
		return fmt.Errorf("setting up SSL: %w", err)
	}

	// Create webhook
	if err := createWebhook(c); err != nil {
		return fmt.Errorf("creating webhook: %w", err)
	}

	return nil
}

func printHelp() {
	fmt.Printf(`Deplobox Installer v%s

Usage: sudo deplobox-installer [options]

Configuration File:
  --config <path>          Path to YAML config file
                          Default locations checked:
                            ./installer/config.yaml
                            /etc/deplobox/installer-config.yaml

Config File Format (all fields optional):
  webhook_url: https://server.example.com
  certbot_email: tech@example.com
  deploy_user: deploybot
  deploy_group: www-data
  projects_root: /var/www/projects
  deplobox_home: /home/deploybot/deplobox
  github_token: ghp_xxxxxxxxxxxx
  project_name: my-app
  owner_repo: username/my-app
  project_domain: app.example.com

Command Line Flags (override config file):
  --webhook-url <url>      Domain where deplobox receives webhooks
  --project-domain <url>   Domain where project is hosted
  --owner-repo <repo>      GitHub owner/repo (e.g., user/repo)
  --project-name <name>    Project slug for webhook path
  --certbot-email <email>  Email for Let's Encrypt
  --deploy-user <user>     Unix user for deployments (default: deploybot)
  --deploy-group <group>   Web group (default: www-data)
  --projects-root <path>   Projects directory (default: /var/www/projects)
  --deplobox-home <path>   Deplobox directory (default: /home/deploybot/deplobox)
  --github-token <token>   GitHub token (can also use GH_TOKEN env var)
  --binary-source <src>    Binary source: 'local' or URL (default: local)
  --webhook-secret <sec>   Webhook secret (auto-generated if not set)
  --git-host-alias <host>  SSH host alias (default: github.<project>)
  --deploy-key-file <name> Deploy key filename (default: <project>.key)
  --verbose               Show verbose output
  --version               Show version
  --help                  Show this help

Environment Variables:
  GH_TOKEN / GITHUB_TOKEN  GitHub Personal Access Token
                          Scopes: repo, admin:public_key, write:repo_hook

Notes:
  - Values are loaded in order: defaults -> config file -> flags -> prompts
  - Only missing required values will be prompted interactively
  - Two domains are used:
      1. webhook_url: Where deplobox service is hosted (reusable)
      2. project_domain: Where this specific project is hosted

Examples:
  # Interactive installation (prompts for everything)
  sudo deplobox-installer

  # Use config file with some prompts
  sudo deplobox-installer --config config.yaml

  # Fully automated (no prompts)
  sudo deplobox-installer --config full-config.yaml

  # Override specific values
  sudo deplobox-installer --config base-config.yaml --project-name new-app

`, version)
}

func printConfigSummary(c *Config) {
	fmt.Println()
	fmt.Println("==========================================")
	fmt.Println("  Configuration Summary")
	fmt.Println("==========================================")
	fmt.Printf("Owner/Repo:      %s\n", c.OwnerRepo)
	fmt.Printf("Project Name:    %s\n", c.ProjectName)
	fmt.Printf("Webhook URL:     %s\n", c.WebhookURL)
	fmt.Printf("Project Domain:  %s\n", c.ProjectDomain)
	fmt.Printf("Deploy User:     %s\n", c.DeployUser)
	fmt.Printf("Deploy Group:    %s\n", c.DeployGroup)
	fmt.Printf("Projects Root:   %s\n", c.ProjectsRoot)
	fmt.Printf("Deplobox Home:   %s\n", c.DeploboxHome)
	fmt.Printf("Git Host Alias:  %s\n", c.GitHostAlias)
	fmt.Printf("Deploy Key File: %s\n", c.DeployKeyFile)
	if c.CertbotEmail != "" {
		fmt.Printf("Certbot Email:   %s\n", c.CertbotEmail)
	} else {
		fmt.Printf("Certbot Email:   (not set - SSL setup will be skipped)\n")
	}
	if c.GitHubToken != "" {
		fmt.Printf("GitHub Token:    %s (set)\n", "***")
	} else {
		fmt.Printf("GitHub Token:    (not set - GitHub automation will be skipped)\n")
	}
	fmt.Printf("Binary Source:   %s\n", c.BinarySource)
}

func printSuccessSummary(c *Config) {
	fmt.Println()
	fmt.Println("==========================================")
	fmt.Printf("  %sDeplobox Installation Complete!%s\n", colorGreen, colorReset)
	fmt.Println("==========================================")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Binary:     %s/deplobox\n", c.DeploboxHome)
	fmt.Printf("  Config:     %s/projects.yaml\n", c.DeploboxHome)
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
	fmt.Printf("  2. Check logs: tail -f %s/deployments.log\n", c.DeploboxHome)
	fmt.Printf("  3. View status: curl %s/status/%s\n", c.WebhookURL, c.ProjectName)
	fmt.Println()
}
