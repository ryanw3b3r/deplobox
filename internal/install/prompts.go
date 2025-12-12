package install

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// PromptForMissingValues interactively prompts for any missing required config values
func PromptForMissingValues(c *Config) error {
	if !isInteractive() {
		return nil
	}

	reader := bufio.NewReader(os.Stdin)

	// Owner/Repo (required)
	if c.OwnerRepo == "" {
		fmt.Println()
		fmt.Println("GitHub repository (owner/repo) for cloning and API calls")
		fmt.Println("Example: ryanw3b3r/deplobox")
		c.OwnerRepo = readValue(reader, "Enter owner/repo", "")
	}

	// Fill derived values from owner/repo
	c.FillDerivedValues()

	// Project Name
	if c.ProjectName == "" {
		fmt.Println()
		fmt.Println("Project slug used in webhook path (/in/<project>) and directory name")
		fmt.Printf("Example: %s\n", suggestProjectName(c.OwnerRepo))
		c.ProjectName = readValue(reader, "Enter project name", suggestProjectName(c.OwnerRepo))
		c.FillDerivedValues()
	}

	// Webhook URL (domain where deplobox receives webhooks)
	if c.WebhookURL == "" {
		fmt.Println()
		fmt.Println("Webhook URL - The domain where deplobox will receive GitHub webhooks")
		fmt.Println("This is where the deplobox service is hosted (reusable across projects)")
		fmt.Println("Example: https://deplobox.ryan-weber.com")
		c.WebhookURL = readValue(reader, "Enter webhook URL", fmt.Sprintf("https://deplobox.%s.com", c.ProjectName))
	}

	// Project Domain (domain where the project itself is hosted)
	if c.ProjectDomain == "" {
		fmt.Println()
		fmt.Println("Project Domain - The domain where THIS project will be hosted")
		fmt.Println("This can be different from the webhook URL")
		fmt.Println("Example: api.example.com")
		c.ProjectDomain = readValue(reader, "Enter project domain", fmt.Sprintf("%s.example.com", c.ProjectName))
	}

	// Deploy User
	if c.DeployUser == "" {
		fmt.Println()
		fmt.Println("Unix user that will own deployments and run the systemd service")
		fmt.Println("Example: deploybot")
		c.DeployUser = readValue(reader, "Enter deploy user", "deploybot")
	}

	// Deploy Group
	if c.DeployGroup == "" {
		fmt.Println()
		fmt.Println("Group that should share access (typically your web server group)")
		fmt.Println("Example: www-data")
		c.DeployGroup = readValue(reader, "Enter deploy group", "www-data")
	}

	// Projects Root
	if c.ProjectsRoot == "" {
		fmt.Println()
		fmt.Println("Filesystem path where project checkouts will live")
		fmt.Println("Example: /var/www/projects")
		c.ProjectsRoot = readValue(reader, "Enter projects root", "/var/www/projects")
	}

	// Deplobox Home
	if c.DeploboxHome == "" {
		fmt.Println()
		fmt.Println("Directory where deplobox binary and config will be stored")
		fmt.Println("Example: /home/deploybot")
		c.DeploboxHome = readValue(reader, "Enter deplobox home", "/home/deploybot")
	}

	// Git Host Alias
	if c.GitHostAlias == "" {
		c.FillDerivedValues()
	}
	fmt.Println()
	fmt.Println("SSH host alias for this deploy key (used in git clone URL)")
	fmt.Printf("Example: github.%s\n", c.ProjectName)
	c.GitHostAlias = readValue(reader, "Enter git host alias", c.GitHostAlias)

	// Deploy Key File
	if c.DeployKeyFile == "" {
		c.FillDerivedValues()
	}
	fmt.Println()
	fmt.Println("Filename for the deploy key (stored in ~/.ssh/)")
	fmt.Printf("Example: %s.key\n", c.ProjectName)
	c.DeployKeyFile = readValue(reader, "Enter deploy key filename", c.DeployKeyFile)

	// Webhook Secret (generate if not provided)
	if c.WebhookSecret == "" {
		secret, err := generateSecret()
		if err != nil {
			return fmt.Errorf("generating webhook secret: %w", err)
		}
		c.WebhookSecret = secret
		fmt.Println()
		fmt.Printf("Generated webhook secret: %s\n", secret)
		fmt.Println("Keep this secret safe - it's used to verify GitHub webhooks")
	}

	// Certbot Email (optional)
	if c.CertbotEmail == "" {
		fmt.Println()
		fmt.Println("Email for Let's Encrypt/Certbot (for SSL certificate expiry notices)")
		fmt.Println("Leave empty to skip SSL setup")
		fmt.Println("Example: tech@example.com")
		c.CertbotEmail = readValue(reader, "Enter email (or leave empty)", "")
	}

	// GitHub Token (optional, can come from env)
	if c.GitHubToken == "" {
		fmt.Println()
		fmt.Println("GitHub Personal Access Token (for automated deploy key and webhook setup)")
		fmt.Println("Leave empty to skip GitHub automation")
		fmt.Println("Scopes needed: repo, admin:public_key, write:repo_hook")
		c.GitHubToken = readValue(reader, "Enter GitHub token (or leave empty)", "")
	}

	return nil
}

// readValue prompts for input with an optional default
func readValue(reader *bufio.Reader, prompt, defaultValue string) string {
	if defaultValue != "" {
		fmt.Printf("%s [%s]: ", prompt, defaultValue)
	} else {
		fmt.Printf("%s: ", prompt)
	}

	input, err := reader.ReadString('\n')
	if err != nil {
		return defaultValue
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return defaultValue
	}
	return input
}

// isInteractive checks if stdin is a terminal
func isInteractive() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// suggestProjectName suggests a project name from owner/repo
func suggestProjectName(ownerRepo string) string {
	parts := strings.Split(ownerRepo, "/")
	if len(parts) == 2 {
		return parts[1]
	}
	return "project-one"
}

// generateSecret generates a random webhook secret
func generateSecret() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
