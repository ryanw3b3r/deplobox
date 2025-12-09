package install

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"deplobox/pkg/templates"
)

// installService creates and starts the systemd service
func installService(c *Config) error {
	if !hasSystemd {
		printSuccess("Skipping service install (no systemd)...")
		return nil
	}

	servicePath := "/etc/systemd/system/deplobox.service"

	// Check if service already exists
	serviceExists := false
	if _, err := os.Stat(servicePath); err == nil {
		serviceExists = true
	}

	// Check if service is running
	serviceRunning := false
	if err := runCmdQuiet("systemctl", "is-active", "deplobox"); err == nil {
		serviceRunning = true
	}

	if serviceExists {
		printSuccess("Deplobox service already exists...")

		// Service exists, just restart it to pick up new projects
		if serviceRunning {
			fmt.Printf("%-70s", "Restarting deplobox service for new config...")
			if err := runCmdQuiet("systemctl", "restart", "deplobox"); err != nil {
				printError("")
				fmt.Println("Failed to restart service. Check: systemctl status deplobox")
				return err
			}
			fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
		} else {
			// Service exists but not running, start it
			if err := runCmd("Starting deplobox service", "systemctl", "start", "deplobox"); err != nil {
				fmt.Println("Service failed to start. Check logs with: journalctl -u deplobox -n 50")
				return err
			}
		}

		// Verify service is running
		exec.Command("sleep", "2").Run()
		if err := runCmdQuiet("systemctl", "is-active", "deplobox"); err == nil {
			printSuccess("Deplobox service is running...")
		} else {
			printWarn("Deplobox service status check...")
		}

		return nil
	}

	// Service doesn't exist, create it
	fmt.Printf("%-70s", "Creating systemd unit file...")

	// Render systemd service from template
	logFile := filepath.Join(c.DeploboxHome, "deployments.log")
	dbPath := filepath.Join(c.DeploboxHome, "deployments.db")

	serviceContent, err := templates.RenderSystemdService(
		c.DeployUser,
		c.DeployGroup,
		c.DeploboxHome,
		c.DeploboxHome,
		logFile,
		dbPath,
	)
	if err != nil {
		printError("")
		return fmt.Errorf("rendering systemd template: %w", err)
	}

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		printError("")
		return fmt.Errorf("writing service file: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)

	if err := runCmd("Reloading systemd units", "systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := runCmd("Enabling deplobox service", "systemctl", "enable", "deplobox"); err != nil {
		return err
	}
	if err := runCmd("Starting deplobox service", "systemctl", "start", "deplobox"); err != nil {
		fmt.Println("Service failed to start. Check logs with: journalctl -u deplobox -n 50")
		return err
	}

	// Give service a moment to start
	fmt.Printf("Waiting for service to start...\n")
	exec.Command("sleep", "2").Run()

	// Check service status
	if err := runCmdQuiet("systemctl", "is-active", "deplobox"); err == nil {
		printSuccess("Verifying deplobox service is running...")
	} else {
		printWarn("Deplobox service status check...")
		fmt.Println("  Service may not be running. Check: systemctl status deplobox")
	}

	return nil
}
