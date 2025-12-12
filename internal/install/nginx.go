package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"deplobox/pkg/templates"
)

// installNginx configures nginx reverse proxy for deplobox
func installNginx(c *Config) error {
	if !hasSystemd {
		printSuccess("Skipping nginx setup (no systemd)...")
		return nil
	}

	// Check if nginx is available
	if _, err := exec.LookPath("nginx"); err != nil {
		printWarn("Nginx not found, skipping configuration...")
		return nil
	}

	// Use project domain as the config filename
	configName := c.ProjectDomain
	sitePath := filepath.Join("/etc/nginx/sites-available", configName)
	enabledPath := filepath.Join("/etc/nginx/sites-enabled", configName)

	// Clean up old "deplobox" config if it exists and is not the current project
	oldConfigPath := "/etc/nginx/sites-available/deplobox"
	oldEnabledPath := "/etc/nginx/sites-enabled/deplobox"
	if configName != "deplobox" {
		if _, err := os.Stat(oldConfigPath); err == nil {
			printWarn("Removing old 'deplobox' nginx config...")
			os.Remove(oldEnabledPath)
			os.Remove(oldConfigPath)
		}
	}

	fmt.Printf("%-70s", fmt.Sprintf("Creating nginx config for %s...", configName))

	// Render nginx config from template
	nginxConfig, err := templates.RenderNginxSite(c.ProjectDomain)
	if err != nil {
		printError("")
		return fmt.Errorf("rendering nginx template: %w", err)
	}

	if err := os.WriteFile(sitePath, []byte(nginxConfig), 0644); err != nil {
		printError("")
		return fmt.Errorf("writing nginx config: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)

	// Create symlink if it doesn't exist
	if _, err := os.Lstat(enabledPath); os.IsNotExist(err) {
		if err := runCmd("Linking nginx site config", "ln", "-sf", sitePath, enabledPath); err != nil {
			return err
		}
	} else {
		printSuccess("Nginx site already enabled...")
	}

	if err := runCmd("Testing nginx configuration", "nginx", "-t"); err != nil {
		return err
	}

	return runCmd("Reloading nginx", "systemctl", "reload", "nginx")
}
