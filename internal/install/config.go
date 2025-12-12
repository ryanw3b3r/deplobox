package install

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all installer configuration
type Config struct {
	// Reusable across projects (typically in config file)
	WebhookURL   string `yaml:"webhook_url"`
	CertbotEmail string `yaml:"certbot_email"`
	DeployUser   string `yaml:"deploy_user"`
	DeployGroup  string `yaml:"deploy_group"`
	ProjectsRoot string `yaml:"projects_root"`
	DeploboxHome string `yaml:"deplobox_home"`
	GitHubToken  string `yaml:"github_token"`
	BinarySource string `yaml:"binary_source"`

	// Project-specific (asked during install if not in config)
	ProjectName   string `yaml:"project_name"`
	OwnerRepo     string `yaml:"owner_repo"`
	ProjectDomain string `yaml:"project_domain"`

	// Derived/generated fields
	WebhookSecret string `yaml:"webhook_secret"`
	GitHostAlias  string `yaml:"git_host_alias"`
	DeployKeyFile string `yaml:"deploy_key_file"`
	Verbose       bool   `yaml:"verbose"`
}

// NewConfig creates a new config with defaults
func NewConfig() *Config {
	return &Config{
		DeployUser:   "deploybot",
		DeployGroup:  "www-data",
		ProjectsRoot: "/var/www/projects",
		DeploboxHome: "/home/deploybot/deplobox",
		BinarySource: "local",
		Verbose:      false,
	}
}

// LoadFromFile loads config from a YAML file
func (c *Config) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Config file is optional
		}
		return fmt.Errorf("reading config file: %w", err)
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return fmt.Errorf("parsing config file: %w", err)
	}

	return nil
}

// SetFromFlags updates config from command line flags
func (c *Config) SetFromFlags(flags map[string]string) {
	for key, value := range flags {
		if value == "" {
			continue
		}
		switch key {
		case "webhook-url":
			c.WebhookURL = value
		case "certbot-email":
			c.CertbotEmail = value
		case "deploy-user":
			c.DeployUser = value
		case "deploy-group":
			c.DeployGroup = value
		case "projects-root":
			c.ProjectsRoot = value
		case "deplobox-home":
			c.DeploboxHome = value
		case "github-token":
			c.GitHubToken = value
		case "binary-source":
			c.BinarySource = value
		case "project-name":
			c.ProjectName = value
		case "owner-repo":
			c.OwnerRepo = value
		case "project-domain":
			c.ProjectDomain = value
		case "webhook-secret":
			c.WebhookSecret = value
		case "git-host-alias":
			c.GitHostAlias = value
		case "deploy-key-file":
			c.DeployKeyFile = value
		}
	}
}

// LoadFromEnv loads GitHub token from environment if not already set
func (c *Config) LoadFromEnv() {
	if c.GitHubToken == "" {
		if token := os.Getenv("GH_TOKEN"); token != "" {
			c.GitHubToken = token
		} else if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			c.GitHubToken = token
		}
	}
}

// FillDerivedValues sets derived values based on other config
func (c *Config) FillDerivedValues() {
	// Extract repo name from owner/repo
	if c.OwnerRepo != "" && c.ProjectName == "" {
		parts := strings.Split(c.OwnerRepo, "/")
		if len(parts) == 2 {
			c.ProjectName = parts[1]
		}
	}

	// Set git host alias if not set
	if c.GitHostAlias == "" && c.ProjectName != "" {
		c.GitHostAlias = fmt.Sprintf("github.%s", c.ProjectName)
	}

	// Set deploy key file if not set
	if c.DeployKeyFile == "" && c.ProjectName != "" {
		c.DeployKeyFile = fmt.Sprintf("%s.key", c.ProjectName)
	}
}

// Validate ensures all required fields are set
func (c *Config) Validate() error {
	var missing []string

	if c.OwnerRepo == "" {
		missing = append(missing, "owner-repo")
	}
	if c.ProjectName == "" {
		missing = append(missing, "project-name")
	}
	if c.WebhookURL == "" {
		missing = append(missing, "webhook-url")
	}
	if c.ProjectDomain == "" {
		missing = append(missing, "project-domain")
	}
	if c.DeployUser == "" {
		missing = append(missing, "deploy-user")
	}
	if c.DeployGroup == "" {
		missing = append(missing, "deploy-group")
	}
	if c.ProjectsRoot == "" {
		missing = append(missing, "projects-root")
	}
	if c.DeploboxHome == "" {
		missing = append(missing, "deplobox-home")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration: %s", strings.Join(missing, ", "))
	}

	// Validate owner/repo format
	if !strings.Contains(c.OwnerRepo, "/") {
		return fmt.Errorf("owner-repo must be in format 'owner/repo', got: %s", c.OwnerRepo)
	}

	return nil
}

// GetWebhookDomain extracts the domain from webhook URL
func (c *Config) GetWebhookDomain() string {
	// Remove https:// or http://
	domain := strings.TrimPrefix(c.WebhookURL, "https://")
	domain = strings.TrimPrefix(domain, "http://")
	// Remove any path
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}
	return domain
}

// SearchConfigPaths returns the default config file search paths
func SearchConfigPaths() []string {
	return []string{
		"./config/installer.yaml",
		"/etc/deplobox/installer.yaml",
		"./installer-config.yaml",
	}
}
