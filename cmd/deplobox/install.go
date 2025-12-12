package main

import (
	"fmt"
	"os"

	"deplobox/internal/install"
	"deplobox/pkg/fileutil"

	"github.com/spf13/cobra"
)

var (
	installConfigFile string
	webhookURL        string
	certbotEmail      string
	deployUser        string
	deployGroup       string
	projectsRoot      string
	deploboxHome      string
	githubToken       string
	projectName       string
	ownerRepo         string
	projectDomain     string
	webhookSecret     string
	gitHostAlias      string
	deployKeyFile     string
	installVerbose    bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install and configure deplobox on a server",
	Long: `Install and configure deplobox on a server.

This command automates the entire setup process including:
- Package installation (git, nginx, certbot, etc.)
- User and directory setup
- SSH key generation
- GitHub deploy key upload (optional)
- Repository cloning
- Systemd service installation
- Nginx reverse proxy configuration
- SSL certificate setup (optional)
- GitHub webhook creation (optional)`,
	RunE: runInstall,
}

func init() {
	// Config file flag
	installCmd.Flags().StringVarP(&installConfigFile, "config", "c", "", "Path to installer config file")

	// Reusable flags
	installCmd.Flags().StringVar(&webhookURL, "webhook-url", "", "Webhook URL (where deplobox is hosted)")
	installCmd.Flags().StringVar(&certbotEmail, "certbot-email", "", "Email for Let's Encrypt")
	installCmd.Flags().StringVar(&deployUser, "deploy-user", "", "Deploy user (default: deploybot)")
	installCmd.Flags().StringVar(&deployGroup, "deploy-group", "", "Web group (default: www-data)")
	installCmd.Flags().StringVar(&projectsRoot, "projects-root", "", "Projects directory")
	installCmd.Flags().StringVar(&deploboxHome, "deplobox-home", "", "Deplobox directory")
	installCmd.Flags().StringVar(&githubToken, "github-token", "", "GitHub token for automation")

	// Project-specific flags
	installCmd.Flags().StringVar(&projectName, "project-name", "", "Project slug")
	installCmd.Flags().StringVar(&ownerRepo, "owner-repo", "", "GitHub owner/repo")
	installCmd.Flags().StringVar(&projectDomain, "project-domain", "", "Project domain (where project is hosted)")

	// Advanced flags
	installCmd.Flags().StringVar(&webhookSecret, "webhook-secret", "", "Webhook secret (generated if not provided)")
	installCmd.Flags().StringVar(&gitHostAlias, "git-host-alias", "", "SSH host alias")
	installCmd.Flags().StringVar(&deployKeyFile, "deploy-key-file", "", "Deploy key filename")

	// Verbose flag
	installCmd.Flags().BoolVarP(&installVerbose, "verbose", "v", false, "Verbose output")
}

func runInstall(cmd *cobra.Command, args []string) error {
	// Check if running as root
	if os.Geteuid() != 0 {
		return fmt.Errorf("installer must be run as root (use sudo)")
	}

	// Create config with defaults
	config := install.NewConfig()

	// Load from config file if specified or auto-detect
	if installConfigFile == "" {
		searchPaths := install.SearchConfigPaths()
		installConfigFile = fileutil.SearchPathsOptional(searchPaths)
	}

	if installConfigFile != "" {
		fmt.Printf("Loading config from: %s\n", installConfigFile)
		if err := config.LoadFromFile(installConfigFile); err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}
	}

	// Override with command line flags
	flags := map[string]string{
		"webhook-url":     webhookURL,
		"certbot-email":   certbotEmail,
		"deploy-user":     deployUser,
		"deploy-group":    deployGroup,
		"projects-root":   projectsRoot,
		"deplobox-home":   deploboxHome,
		"github-token":    githubToken,
		"project-name":    projectName,
		"owner-repo":      ownerRepo,
		"project-domain":  projectDomain,
		"webhook-secret":  webhookSecret,
		"git-host-alias":  gitHostAlias,
		"deploy-key-file": deployKeyFile,
	}
	config.SetFromFlags(flags)

	// Load GitHub token from environment if not set
	config.LoadFromEnv()

	// Fill derived values (project name, git host alias, deploy key file)
	config.FillDerivedValues()

	// Set verbose flag
	config.Verbose = installVerbose

	// Prompt for any missing required values (interactive mode)
	if err := install.PromptForMissingValues(config); err != nil {
		return fmt.Errorf("failed to gather configuration: %w", err)
	}

	// Create installer and run
	installer := install.New(config, installVerbose)
	if err := installer.Run(); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	return nil
}
