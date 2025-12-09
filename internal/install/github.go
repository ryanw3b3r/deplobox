package install

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// createGitHubClient creates an authenticated GitHub client
func createGitHubClient(token string) *github.Client {
	if token == "" {
		return nil
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc)
}

// uploadDeployKey uploads the SSH deploy key to GitHub
func uploadDeployKey(c *Config) error {
	if c.GitHubToken == "" {
		fmt.Printf("GitHub automation unavailable; skip deploy key upload\n")
		return nil
	}

	client := createGitHubClient(c.GitHubToken)
	if client == nil {
		fmt.Printf("GitHub automation unavailable; skip deploy key upload\n")
		return nil
	}

	// Read public key
	pubKeyPath := filepath.Join("/home", c.DeployUser, ".ssh", c.DeployKeyFile+".pub")
	pubKeyBytes, err := os.ReadFile(pubKeyPath)
	if err != nil {
		fmt.Printf("Deploy key %s not found; skipping upload\n", pubKeyPath)
		return nil
	}

	pubKeyContent := strings.TrimSpace(string(pubKeyBytes))

	// Parse owner and repo
	parts := strings.Split(c.OwnerRepo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid owner/repo format: %s", c.OwnerRepo)
	}
	owner, repo := parts[0], parts[1]

	ctx := context.Background()

	// Check if key already exists
	keys, _, err := client.Repositories.ListKeys(ctx, owner, repo, nil)
	if err != nil {
		return fmt.Errorf("listing deploy keys: %w", err)
	}

	for _, key := range keys {
		if key.Key != nil && strings.TrimSpace(*key.Key) == pubKeyContent {
			printSuccess("Deploy key already exists on GitHub...")
			return nil
		}
	}

	// Upload new key
	fmt.Printf("%-70s", "Uploading deploy key to GitHub...")

	title := fmt.Sprintf("%s-deplobox", c.ProjectName)
	readOnly := false
	keyReq := &github.Key{
		Title:    &title,
		Key:      &pubKeyContent,
		ReadOnly: &readOnly,
	}

	_, _, err = client.Repositories.CreateKey(ctx, owner, repo, keyReq)
	if err != nil {
		// Check if error is because key already exists
		if strings.Contains(err.Error(), "key is already in use") {
			printWarn("Deploy key may already exist")
			return nil
		}
		printError("")
		return fmt.Errorf("creating deploy key: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	return nil
}

// createWebhook creates a GitHub webhook for the repository
func createWebhook(c *Config) error {
	if c.GitHubToken == "" {
		fmt.Printf("GitHub automation unavailable; skip webhook creation\n")
		return nil
	}

	client := createGitHubClient(c.GitHubToken)
	if client == nil {
		fmt.Printf("GitHub automation unavailable; skip webhook creation\n")
		return nil
	}

	// Parse owner and repo
	parts := strings.Split(c.OwnerRepo, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid owner/repo format: %s", c.OwnerRepo)
	}
	owner, repo := parts[0], parts[1]

	ctx := context.Background()

	// Build webhook URL
	webhookURL := fmt.Sprintf("%s/in/%s", c.WebhookURL, c.ProjectName)

	// Check if webhook already exists
	hooks, _, err := client.Repositories.ListHooks(ctx, owner, repo, nil)
	if err != nil {
		return fmt.Errorf("listing webhooks: %w", err)
	}

	for _, hook := range hooks {
		if hook.Config != nil {
			if url, ok := hook.Config["url"].(string); ok && url == webhookURL {
				printSuccess("Webhook already exists on GitHub...")
				return nil
			}
		}
	}

	// Create new webhook
	fmt.Printf("%-70s", "Creating GitHub webhook...")

	hookConfig := map[string]interface{}{
		"url":          webhookURL,
		"content_type": "json",
		"secret":       c.WebhookSecret,
		"insecure_ssl": "0",
	}

	active := true
	hookReq := &github.Hook{
		Events: []string{"push"},
		Active: &active,
		Config: hookConfig,
	}

	_, _, err = client.Repositories.CreateHook(ctx, owner, repo, hookReq)
	if err != nil {
		printError("")
		return fmt.Errorf("creating webhook: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	return nil
}
