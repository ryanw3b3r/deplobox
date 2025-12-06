package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	aptUpdated  = false
	hasSystemd  = false
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

func init() {
	// Check if output is a terminal
	if stat, err := os.Stdout.Stat(); err == nil {
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Not a terminal, disable colors
			colorGreen = ""
			colorRed = ""
			colorYellow = ""
			colorReset = ""
		}
	}

	// Check if systemd is available
	hasSystemd = checkSystemd()
}

// checkSystemd checks if systemd is available on the system
func checkSystemd() bool {
	if _, err := os.Stat("/run/systemd/system"); err != nil {
		return false
	}
	_, err := exec.LookPath("systemctl")
	return err == nil
}

// runCmd executes a command and shows progress
func runCmd(description string, name string, args ...string) error {
	fmt.Printf("%-70s", description+"...")

	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
		fmt.Printf("%s\n", string(output))
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	return nil
}

// runCmdQuiet executes a command without showing output
func runCmdQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

// ensurePackage ensures a package is installed via apt
func ensurePackage(pkg string) error {
	// Check if already installed
	if err := runCmdQuiet("dpkg", "-s", pkg); err == nil {
		fmt.Printf("%-70s%s[OK]%s\n", fmt.Sprintf("Package %s already installed...", pkg), colorGreen, colorReset)
		return nil
	}

	// Update apt if not already done
	if !aptUpdated {
		if err := runCmd("Updating apt package index", "apt-get", "update"); err != nil {
			return err
		}
		aptUpdated = true
	}

	// Install package
	return runCmd(fmt.Sprintf("Installing package %s", pkg), "apt-get", "install", "-y", pkg)
}

// ensureUser creates the deploy user if it doesn't exist
func ensureUser(c *Config) error {
	// Check if user exists
	if err := runCmdQuiet("id", "-u", c.DeployUser); err == nil {
		fmt.Printf("%-70s%s[OK]%s\n", fmt.Sprintf("User %s already exists...", c.DeployUser), colorGreen, colorReset)
	} else {
		if err := runCmd(fmt.Sprintf("Creating deploy user %s", c.DeployUser), "adduser", "--disabled-password", "--gecos", "", c.DeployUser); err != nil {
			return err
		}
	}

	// Add user to group
	return runCmd(fmt.Sprintf("Adding %s to group %s", c.DeployUser, c.DeployGroup), "usermod", "-a", "-G", c.DeployGroup, c.DeployUser)
}

// setupProjectsDir creates and configures the projects directory
func setupProjectsDir(c *Config) error {
	if err := runCmd(fmt.Sprintf("Ensuring projects root %s", c.ProjectsRoot), "mkdir", "-p", c.ProjectsRoot); err != nil {
		return err
	}
	return runCmd(fmt.Sprintf("Setting ownership on %s", c.ProjectsRoot), "chown", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), c.ProjectsRoot)
}

// setupDeploboxHome creates deplobox home directory structure
func setupDeploboxHome(c *Config) error {
	if err := runCmd(fmt.Sprintf("Creating deplobox home %s", c.DeploboxHome), "mkdir", "-p", c.DeploboxHome); err != nil {
		return err
	}
	configDir := filepath.Join(c.DeploboxHome, "config")
	if err := runCmd("Creating deplobox config dir", "mkdir", "-p", configDir); err != nil {
		return err
	}
	return runCmd(fmt.Sprintf("Setting ownership on %s", c.DeploboxHome), "chown", "-R", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), c.DeploboxHome)
}

// setupSSH configures SSH keys and config for git access
func setupSSH(c *Config) error {
	sshDir := filepath.Join("/home", c.DeployUser, ".ssh")
	keyPath := filepath.Join(sshDir, c.DeployKeyFile)
	pubKeyPath := keyPath + ".pub"

	// Create SSH directory
	if err := runCmd(fmt.Sprintf("Ensuring SSH dir %s", sshDir), "mkdir", "-p", sshDir); err != nil {
		return err
	}
	if err := runCmd(fmt.Sprintf("Setting permissions on %s", sshDir), "chmod", "700", sshDir); err != nil {
		return err
	}

	// Generate SSH key if it doesn't exist
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if err := generateSSHKey(keyPath, c.ProjectName); err != nil {
			return fmt.Errorf("generating SSH key: %w", err)
		}
		fmt.Printf("%-70s%s[OK]%s\n", fmt.Sprintf("Generated deploy key %s...", c.DeployKeyFile), colorGreen, colorReset)

		if err := runCmd("Setting permissions on deploy key", "chmod", "600", keyPath, pubKeyPath); err != nil {
			return err
		}
	} else {
		fmt.Printf("%-70s%s[OK]%s\n", "Deploy key already present; skipping generation...", colorGreen, colorReset)
	}

	// Configure SSH config
	configPath := filepath.Join(sshDir, "config")
	if err := configureSSHConfig(configPath, c.GitHostAlias, keyPath); err != nil {
		return err
	}

	// Set permissions
	if err := runCmd("Setting permissions on SSH config", "chmod", "600", configPath); err != nil {
		return err
	}
	return runCmd("Setting ownership on SSH files", "chown", "-R", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), sshDir)
}

// generateSSHKey generates an ED25519 SSH key pair
func generateSSHKey(path, comment string) error {
	// Use ssh-keygen command as it's more reliable
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", path, "-C", comment)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ssh-keygen failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// configureSSHConfig adds or updates SSH config for git host alias
func configureSSHConfig(configPath, hostAlias, keyPath string) error {
	// Read existing config
	var existingContent string
	if data, err := os.ReadFile(configPath); err == nil {
		existingContent = string(data)
	}

	// Check if host alias already exists
	if strings.Contains(existingContent, fmt.Sprintf("Host %s", hostAlias)) {
		fmt.Printf("%-70s%s[OK]%s\n", fmt.Sprintf("SSH config for %s already exists...", hostAlias), colorGreen, colorReset)
		return nil
	}

	// Append new host config
	fmt.Printf("%-70s", fmt.Sprintf("Adding SSH config for %s...", hostAlias))
	sshConfig := fmt.Sprintf(`
Host %s
    HostName github.com
    IdentityFile %s
    IdentitiesOnly yes
`, hostAlias, keyPath)

	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
		return fmt.Errorf("opening SSH config: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(sshConfig); err != nil {
		fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
		return fmt.Errorf("writing SSH config: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	return nil
}

// generateSecret generates a random webhook secret
func generateSecret() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ensureGitClone clones the repository if it doesn't exist
func ensureGitClone(c *Config) error {
	targetPath := filepath.Join(c.ProjectsRoot, c.ProjectName)
	gitDir := filepath.Join(targetPath, ".git")

	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		// Repository doesn't exist, clone it
		cloneURL := fmt.Sprintf("git@%s:%s.git", c.GitHostAlias, c.OwnerRepo)

		fmt.Printf("%-70s", fmt.Sprintf("Cloning %s into %s...", c.OwnerRepo, targetPath))

		cmd := exec.Command("sudo", "-u", c.DeployUser, "-H", "git", "clone", cloneURL, c.ProjectName)
		cmd.Dir = c.ProjectsRoot
		output, err := cmd.CombinedOutput()

		if err != nil {
			fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
			fmt.Printf("%s\n", string(output))
			return fmt.Errorf("cloning repository: %w", err)
		}

		fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	} else {
		// Repository already exists
		fmt.Println()
		fmt.Printf("%sRepository already exists at %s%s\n", colorYellow, targetPath, colorReset)
		fmt.Println("What would you like to do?")
		fmt.Println("  1. Keep existing repository (skip)")
		fmt.Println("  2. Delete and re-clone (fresh start)")
		fmt.Println("  3. Pull latest changes")
		fmt.Printf("Enter choice [1]: ")

		var response string
		fmt.Scanln(&response)
		if response == "" {
			response = "1"
		}

		switch response {
		case "1":
			fmt.Printf("%-70s%s[SKIP]%s\n", "Keeping existing repository...", colorYellow, colorReset)
		case "2":
			// Delete and re-clone
			fmt.Printf("%-70s", "Deleting existing repository...")
			if err := os.RemoveAll(targetPath); err != nil {
				fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
				return fmt.Errorf("deleting existing repository: %w", err)
			}
			fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)

			cloneURL := fmt.Sprintf("git@%s:%s.git", c.GitHostAlias, c.OwnerRepo)
			fmt.Printf("%-70s", fmt.Sprintf("Cloning %s...", c.OwnerRepo))

			cmd := exec.Command("sudo", "-u", c.DeployUser, "-H", "git", "clone", cloneURL, c.ProjectName)
			cmd.Dir = c.ProjectsRoot
			output, err := cmd.CombinedOutput()

			if err != nil {
				fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
				fmt.Printf("%s\n", string(output))
				return fmt.Errorf("cloning repository: %w", err)
			}
			fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
		case "3":
			// Pull latest changes
			fmt.Printf("%-70s", "Pulling latest changes...")
			cmd := exec.Command("sudo", "-u", c.DeployUser, "-H", "git", "pull")
			cmd.Dir = targetPath
			output, err := cmd.CombinedOutput()

			if err != nil {
				fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
				fmt.Printf("%s\n", string(output))
				return fmt.Errorf("pulling changes: %w", err)
			}
			fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
		default:
			fmt.Printf("%-70s%s[SKIP]%s\n", "Invalid choice, keeping existing repository...", colorYellow, colorReset)
		}
	}

	return runCmd(fmt.Sprintf("Setting ownership on %s", targetPath), "chown", "-R", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), targetPath)
}

// writeProjectsYAML creates or updates the projects.yaml config file
func writeProjectsYAML(c *Config) error {
	configPath := filepath.Join(c.DeploboxHome, "projects.yaml")
	projectPath := filepath.Join(c.ProjectsRoot, c.ProjectName)

	// Define the project structure
	type ProjectConfig struct {
		Path              string   `yaml:"path"`
		Secret            string   `yaml:"secret"`
		Branch            string   `yaml:"branch"`
		PullTimeout       int      `yaml:"pull_timeout"`
		PostDeployTimeout int      `yaml:"post_deploy_timeout"`
		PostDeploy        []string `yaml:"post_deploy"`
	}

	type ProjectsFile struct {
		Projects map[string]ProjectConfig `yaml:"projects"`
	}

	projectsData := ProjectsFile{
		Projects: make(map[string]ProjectConfig),
	}

	// Load existing config if it exists
	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, &projectsData); err != nil {
			return fmt.Errorf("parsing existing projects.yaml: %w", err)
		}

		// Check if project already exists
		if existing, exists := projectsData.Projects[c.ProjectName]; exists {
			fmt.Println()
			fmt.Printf("%sProject '%s' already exists in config%s\n", colorYellow, c.ProjectName, colorReset)
			fmt.Printf("Current settings:\n")
			fmt.Printf("  Path:   %s\n", existing.Path)
			fmt.Printf("  Branch: %s\n", existing.Branch)
			fmt.Println()
			fmt.Printf("Do you want to update it? [y/N]: ")
			var response string
			fmt.Scanln(&response)
			if response != "y" && response != "Y" {
				fmt.Printf("%-70s%s[SKIP]%s\n", "Keeping existing project config...", colorYellow, colorReset)
				return nil
			}
			fmt.Printf("%-70s", "Updating project in projects.yaml...")
		} else {
			fmt.Printf("%-70s", fmt.Sprintf("Adding project '%s' to projects.yaml...", c.ProjectName))
		}
	} else {
		fmt.Printf("%-70s", "Creating projects.yaml config...")
	}

	// Add or update project
	projectsData.Projects[c.ProjectName] = ProjectConfig{
		Path:              projectPath,
		Secret:            c.WebhookSecret,
		Branch:            "main",
		PullTimeout:       60,
		PostDeployTimeout: 300,
		PostDeploy:        []string{},
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(&projectsData)
	if err != nil {
		fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
		return fmt.Errorf("marshaling projects.yaml: %w", err)
	}

	// Write config
	if err := os.WriteFile(configPath, yamlBytes, 0640); err != nil {
		fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
		return fmt.Errorf("writing projects.yaml: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)

	if err := runCmd(fmt.Sprintf("Setting ownership on %s", configPath), "chown", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), configPath); err != nil {
		return err
	}
	return runCmd(fmt.Sprintf("Setting permissions on %s", configPath), "chmod", "640", configPath)
}

// deployBinary copies or downloads the deplobox binary
func deployBinary(c *Config) error {
	targetPath := filepath.Join(c.DeploboxHome, "deplobox")

	// Check if binary already exists
	if stat, err := os.Stat(targetPath); err == nil && stat.Mode().IsRegular() {
		fmt.Println()
		fmt.Printf("%sDeplobox binary already exists at %s%s\n", colorYellow, targetPath, colorReset)
		fmt.Println("Do you want to update it? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" {
			fmt.Printf("%-70s%s[SKIP]%s\n", "Keeping existing binary...", colorYellow, colorReset)
			return nil
		}
		// User wants to update, backup old binary
		backupPath := targetPath + ".backup"
		fmt.Printf("%-70s", "Backing up existing binary...")
		if err := runCmdQuiet("cp", targetPath, backupPath); err == nil {
			fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
		} else {
			fmt.Printf("%s[WARN]%s\n", colorYellow, colorReset)
		}
	}

	if c.BinarySource == "local" {
		// Determine architecture
		arch := os.Getenv("HOSTTYPE")
		if arch == "" {
			// Fallback to uname
			cmd := exec.Command("uname", "-m")
			output, err := cmd.Output()
			if err != nil {
				return fmt.Errorf("detecting architecture: %w", err)
			}
			arch = strings.TrimSpace(string(output))
		}

		var binaryName string
		switch arch {
		case "x86_64":
			binaryName = "deplobox-linux-amd64"
		case "aarch64":
			binaryName = "deplobox-linux-arm64"
		default:
			return fmt.Errorf("unsupported architecture: %s", arch)
		}

		// Get script directory
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("getting executable path: %w", err)
		}
		scriptDir := filepath.Dir(executable)
		sourcePath := filepath.Join(scriptDir, binaryName)

		if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
			return fmt.Errorf("binary not found: %s\nPlease build binaries first or specify --binary-source with URL", sourcePath)
		}

		if err := runCmd(fmt.Sprintf("Copying %s to %s", binaryName, targetPath), "cp", sourcePath, targetPath); err != nil {
			return err
		}
	} else {
		// Download from URL
		if err := runCmd("Downloading deplobox binary", "curl", "-fsSL", "-o", targetPath, c.BinarySource); err != nil {
			return err
		}
	}

	if err := runCmd("Setting executable permissions", "chmod", "755", targetPath); err != nil {
		return err
	}
	return runCmd("Setting ownership on binary", "chown", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), targetPath)
}

// installService creates and starts the systemd service
func installService(c *Config) error {
	if !hasSystemd {
		fmt.Printf("%-70s%s[OK]%s\n", "Skipping service install (no systemd)...", colorGreen, colorReset)
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
		fmt.Printf("%-70s%s[OK]%s\n", "Deplobox service already exists...", colorGreen, colorReset)

		// Service exists, just restart it to pick up new projects
		if serviceRunning {
			fmt.Printf("%-70s", "Restarting deplobox service for new config...")
			if err := runCmdQuiet("systemctl", "restart", "deplobox"); err != nil {
				fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
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
			fmt.Printf("%-70s%s[OK]%s\n", "Deplobox service is running...", colorGreen, colorReset)
		} else {
			fmt.Printf("%-70s%s[WARN]%s\n", "Deplobox service status check...", colorYellow, colorReset)
		}

		return nil
	}

	// Service doesn't exist, create it
	fmt.Printf("%-70s", "Creating systemd unit file...")

	serviceContent := fmt.Sprintf(`[Unit]
Description=Deplobox GitHub Webhook Receiver (Go)
Documentation=https://github.com/ryanw3b3r/deplobox
After=network.target

[Service]
Type=simple
User=%s
Group=%s
WorkingDirectory=%s

ExecStart=%s/deplobox -config %s/projects.yaml

Restart=always
RestartSec=5

Environment="DEPLOBOX_LOG_FILE=%s/deployments.log"
Environment="DEPLOBOX_DB_PATH=%s/deployments.db"

# Security settings
NoNewPrivileges=true
PrivateTmp=true

[Install]
WantedBy=multi-user.target
`, c.DeployUser, c.DeployGroup, c.DeploboxHome, c.DeploboxHome, c.DeploboxHome, c.DeploboxHome, c.DeploboxHome)

	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
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
		fmt.Printf("%-70s%s[OK]%s\n", "Verifying deplobox service is running...", colorGreen, colorReset)
	} else {
		fmt.Printf("%-70s%s[WARN]%s\n", "Deplobox service status check...", colorYellow, colorReset)
		fmt.Println("  Service may not be running. Check: systemctl status deplobox")
	}

	return nil
}
