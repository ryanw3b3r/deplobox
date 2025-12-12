package install

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"deplobox/internal/security"
	"deplobox/pkg/fileutil"
	"gopkg.in/yaml.v3"
)

var (
	aptUpdated  = false
	hasSystemd  = false
	installLog  *os.File
	installLogW io.Writer
)

func init() {
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

// initInstallLog opens the installation log file for writing
func initInstallLog(logPath string) error {
	// Create or open log file in append mode
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening log file: %w", err)
	}
	installLog = f
	installLogW = f

	// Write installation start marker
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(installLogW, "\n=== Installation started at %s ===\n\n", timestamp)

	return nil
}

// closeInstallLog closes the installation log file
func closeInstallLog() {
	if installLog != nil {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		fmt.Fprintf(installLogW, "\n=== Installation completed at %s ===\n\n", timestamp)
		installLog.Close()
		installLog = nil
		installLogW = nil
	}
}

// logToFile writes a message to the installation log if it's open
func logToFile(format string, args ...interface{}) {
	if installLogW != nil {
		fmt.Fprintf(installLogW, format, args...)
	}
}

// runCmd executes a command and shows progress
func runCmd(description string, name string, args ...string) error {
	fmt.Printf("%-70s", description+"...")

	// Log command to file
	logToFile("[CMD] %s %s\n", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()

	// Log output to file (success or failure)
	if len(output) > 0 {
		logToFile("%s\n", string(output))
	}

	if err != nil {
		printError("")
		fmt.Printf("%s\n", string(output))
		logToFile("[ERROR] Command failed: %v\n\n", err)
		return fmt.Errorf("command failed: %w\nOutput: %s", err, string(output))
	}

	logToFile("[OK] %s\n\n", description)
	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	return nil
}

// runCmdQuiet executes a command without showing output
func runCmdQuiet(name string, args ...string) error {
	// Log command to file
	logToFile("[CMD] %s %s\n", name, strings.Join(args, " "))

	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()

	// Log output to file even if quiet
	if len(output) > 0 {
		logToFile("%s\n", string(output))
	}

	if err != nil {
		logToFile("[ERROR] Command failed: %v\n\n", err)
	} else {
		logToFile("[OK]\n\n")
	}

	return err
}

// ensurePackage ensures a package is installed via apt
func ensurePackage(pkg string) error {
	// Check if already installed
	if err := runCmdQuiet("dpkg", "-s", pkg); err == nil {
		printSuccess(fmt.Sprintf("Package %s already installed...", pkg))
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
		printSuccess(fmt.Sprintf("User %s already exists...", c.DeployUser))
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

// NOTE: setupDeploboxHome function has been removed - deplobox home directory
// and configs will be created by the distribution package, not at runtime

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
		printSuccess(fmt.Sprintf("Generated deploy key %s...", c.DeployKeyFile))

		if err := runCmd("Setting permissions on deploy key", "chmod", "600", keyPath, pubKeyPath); err != nil {
			return err
		}
	} else {
		printSuccess("Deploy key already present; skipping generation...")
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
	// Log to file
	logToFile("[CMD] ssh-keygen -t ed25519 -N \"\" -f %s -C %s\n", path, comment)

	// Use ssh-keygen command as it's more reliable
	cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-f", path, "-C", comment)
	output, err := cmd.CombinedOutput()

	// Log output to file
	if len(output) > 0 {
		logToFile("%s\n", string(output))
	}

	if err != nil {
		logToFile("[ERROR] ssh-keygen failed: %v\n\n", err)
		return fmt.Errorf("ssh-keygen failed: %w\nOutput: %s", err, string(output))
	}

	logToFile("[OK] SSH key generated\n\n")
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
		printSuccess(fmt.Sprintf("SSH config for %s already exists...", hostAlias))
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

	f, err := os.OpenFile(configPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, security.PermSSHKey)
	if err != nil {
		printError("")
		return fmt.Errorf("opening SSH config: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(sshConfig); err != nil {
		printError("")
		return fmt.Errorf("writing SSH config: %w", err)
	}

	// Explicitly set permissions to bypass umask
	if err := os.Chmod(configPath, security.PermSSHKey); err != nil {
		printError("")
		return fmt.Errorf("setting SSH config permissions: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	return nil
}

// ensureGitClone creates zero-downtime deployment structure and clones repository
func ensureGitClone(c *Config) error {
	projectRoot := filepath.Join(c.ProjectsRoot, c.ProjectName)
	sharedDir := filepath.Join(projectRoot, "shared")
	releasesDir := filepath.Join(projectRoot, "releases")
	currentLink := filepath.Join(projectRoot, "current")

	// Check if structure already exists
	if _, err := os.Stat(currentLink); err == nil {
		// Project already deployed
		fmt.Println()
		fmt.Printf("%sProject '%s' is already deployed%s\n", colorYellow, c.ProjectName, colorReset)
		fmt.Println("What would you like to do?")
		fmt.Println("  1. Keep existing deployment (skip)")
		fmt.Println("  2. Create new release")
		fmt.Printf("Enter choice [1]: ")

		var response string
		fmt.Scanln(&response)
		if response == "" {
			response = "1"
		}

		if response == "1" {
			printWarn("Keeping existing deployment...")
			return nil
		}
	}

	// Create project structure
	if err := runCmd(fmt.Sprintf("Creating project root %s", projectRoot), "mkdir", "-p", projectRoot); err != nil {
		return err
	}
	if err := runCmd("Creating shared directory", "mkdir", "-p", sharedDir); err != nil {
		return err
	}
	if err := runCmd("Creating releases directory", "mkdir", "-p", releasesDir); err != nil {
		return err
	}

	// Set ownership before cloning so deploybot user can write to directories
	if err := runCmd(fmt.Sprintf("Setting ownership on %s", projectRoot), "chown", "-R", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), projectRoot); err != nil {
		return err
	}

	// Generate timestamp for release
	timestamp := time.Now().Format("2006-01-02-15-04-05")
	releaseDir := filepath.Join(releasesDir, timestamp)

	// SECURITY: Validate git host alias and project name before constructing clone URL
	if err := security.ValidateProjectName(c.ProjectName); err != nil {
		return fmt.Errorf("invalid project name: %w", err)
	}

	// Build clone URL (using SSH host alias from config)
	cloneURL := fmt.Sprintf("git@%s:%s.git", c.GitHostAlias, c.OwnerRepo)

	// SECURITY: Validate the constructed clone URL
	// While we're using SSH (which is more secure than HTTPS for this case),
	// we still validate the owner/repo format
	if !strings.Contains(c.OwnerRepo, "/") {
		return fmt.Errorf("invalid owner/repo format: %s", c.OwnerRepo)
	}

	fmt.Printf("%-70s", fmt.Sprintf("Cloning %s into release %s...", c.OwnerRepo, timestamp))

	// Log to file
	logToFile("[CMD] sudo -u %s -H git clone %s %s (in %s)\n", c.DeployUser, cloneURL, timestamp, releasesDir)

	cmd := exec.Command("sudo", "-u", c.DeployUser, "-H", "git", "clone", cloneURL, timestamp)
	cmd.Dir = releasesDir
	output, err := cmd.CombinedOutput()

	// Log output to file
	if len(output) > 0 {
		logToFile("%s\n", string(output))
	}

	if err != nil {
		printError("")
		fmt.Printf("%s\n", string(output))
		logToFile("[ERROR] Git clone failed: %v\n\n", err)
		return fmt.Errorf("cloning repository: %w", err)
	}

	logToFile("[OK] Cloning completed\n\n")
	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)

	// Copy shared files to release (if any exist)
	if err := copySharedToRelease(c.DeployUser, sharedDir, releaseDir); err != nil {
		return err
	}

	// Create/update current symlink atomically
	if err := updateCurrentSymlink(currentLink, releaseDir); err != nil {
		return err
	}

	// Set ownership on entire project structure
	return runCmd(fmt.Sprintf("Setting ownership on %s", projectRoot), "chown", "-R", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), projectRoot)
}

// copySharedToRelease copies shared files/folders to release directory
func copySharedToRelease(user, sharedDir, releaseDir string) error {
	// Check if shared directory has any contents
	entries, err := os.ReadDir(sharedDir)
	if err != nil || len(entries) == 0 {
		// No shared files to copy
		return nil
	}

	fmt.Printf("%-70s", "Copying shared files to release...")

	// Log to file
	logToFile("[CMD] sudo -u %s rsync -a %s/ %s/\n", user, sharedDir, releaseDir)

	// Use rsync to copy/merge shared files
	cmd := exec.Command("sudo", "-u", user, "rsync", "-a", sharedDir+"/", releaseDir+"/")
	output, err := cmd.CombinedOutput()

	// Log output to file
	if len(output) > 0 {
		logToFile("%s\n", string(output))
	}

	if err != nil {
		printError("")
		fmt.Printf("%s\n", string(output))
		logToFile("[ERROR] Rsync failed: %v\n\n", err)
		return fmt.Errorf("copying shared files: %w", err)
	}

	logToFile("[OK] Copying shared files completed\n\n")
	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	return nil
}

// updateCurrentSymlink creates or updates the current symlink atomically
func updateCurrentSymlink(currentLink, releaseDir string) error {
	fmt.Printf("%-70s", "Updating current symlink...")

	// Use atomic symlink update from pkg/fileutil
	if err := fileutil.UpdateSymlinkAtomic(currentLink, releaseDir); err != nil {
		printError("")
		return err
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)
	return nil
}

// writeProjectsYAML creates or updates the projects.yaml config file
func writeProjectsYAML(c *Config) error {
	// Use /etc/deplobox for system-wide installation
	configDir := "/etc/deplobox"
	configPath := filepath.Join(configDir, "projects.yaml")
	projectPath := filepath.Join(c.ProjectsRoot, c.ProjectName)

	// Ensure config directory exists
	if err := runCmd(fmt.Sprintf("Ensuring config directory %s", configDir), "mkdir", "-p", configDir); err != nil {
		return err
	}

	// Define the project structure
	type ProjectConfig struct {
		Path                string   `yaml:"path"`
		Secret              string   `yaml:"secret"`
		Branch              string   `yaml:"branch"`
		PullTimeout         int      `yaml:"pull_timeout"`
		PostDeployTimeout   int      `yaml:"post_deploy_timeout"`
		PostDeploy          []string `yaml:"post_deploy"`
		PostActivateTimeout int      `yaml:"post_activate_timeout"`
		PostActivate        []string `yaml:"post_activate"`
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

		// Ensure Projects map is initialized (YAML unmarshaling can create nil map)
		if projectsData.Projects == nil {
			projectsData.Projects = make(map[string]ProjectConfig)
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
				printWarn("Keeping existing project config...")
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
		Path:                projectPath,
		Secret:              c.WebhookSecret,
		Branch:              "main",
		PullTimeout:         60,
		PostDeployTimeout:   300,
		PostDeploy:          []string{},
		PostActivateTimeout: 300,
		PostActivate:        []string{},
	}

	// Marshal to YAML
	yamlBytes, err := yaml.Marshal(&projectsData)
	if err != nil {
		printError("")
		return fmt.Errorf("marshaling projects.yaml: %w", err)
	}

	// Write config with secure permissions
	if err := os.WriteFile(configPath, yamlBytes, security.PermConfigFile); err != nil {
		printError("")
		return fmt.Errorf("writing projects.yaml: %w", err)
	}

	// Explicitly set permissions to bypass umask
	if err := os.Chmod(configPath, security.PermConfigFile); err != nil {
		printError("")
		return fmt.Errorf("setting projects.yaml permissions: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)

	// Set ownership on the entire config directory
	if err := runCmd(fmt.Sprintf("Setting ownership on %s", configDir), "chown", "-R", fmt.Sprintf("%s:%s", c.DeployUser, c.DeployGroup), configDir); err != nil {
		return err
	}
	return nil
}
