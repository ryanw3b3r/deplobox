package main

import (
	"fmt"

	"deplobox/internal/deployment"
	"deplobox/internal/project"

	"github.com/spf13/cobra"
)

const defaultConfigPath = "/etc/deplobox/projects.yaml"

var (
	restoreConfigFile string
)

var restoreCmd = &cobra.Command{
	Use:   "restore PROJECT_NAME",
	Short: "Restore a project to its previous release",
	Long: `Restore a project to its previous release by switching the current symlink.

This command will:
- Read the project configuration from projects.yaml
- Find the previous release (by timestamp)
- Atomically switch the current symlink to the previous release

Example:
  deplobox restore myapp`,
	Args: cobra.ExactArgs(1),
	RunE: runRestore,
}

func init() {
	// Config file flag
	restoreCmd.Flags().StringVarP(&restoreConfigFile, "config", "c", defaultConfigPath, "Path to projects config file")
}

func runRestore(cmd *cobra.Command, args []string) error {
	projectName := args[0]

	// Load project configuration
	_, projects, err := project.LoadConfig(restoreConfigFile)
	if err != nil {
		return fmt.Errorf("failed to load config from %s: %w", restoreConfigFile, err)
	}

	// Find the project
	proj, exists := projects[projectName]
	if !exists {
		return fmt.Errorf("project '%s' not found in config file %s", projectName, restoreConfigFile)
	}

	// Create executor for the project
	executor := deployment.NewExecutor(proj.Path)

	// Restore to previous release
	fmt.Printf("Restoring project '%s' to previous release...\n", projectName)
	oldRelease, newRelease, err := executor.RestorePreviousRelease()
	if err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Printf("\nRestore successful!\n")
	fmt.Printf("  Previous (current): %s\n", oldRelease)
	fmt.Printf("  Restored to:        %s\n", newRelease)
	fmt.Printf("\nThe 'current' symlink now points to: %s\n", newRelease)

	return nil
}
