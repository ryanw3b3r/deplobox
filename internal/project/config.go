package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	MinSecretLength            = 32
	DefaultPullTimeout         = 60
	DefaultPostDeployTimeout   = 300
	DefaultPostActivateTimeout = 300
)

var ForbiddenSecrets = map[string]bool{
	"replace-with-secret":     true,
	"github-webhook-password": true,
	"topsecret":               true,
	"secret":                  true,
	"password":                true,
	"changeme":                true,
}

// LoadConfig loads and validates the configuration from a YAML file
func LoadConfig(configPath string) (*Config, map[string]*Project, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}

	// Initialize Projects map if it's nil (happens with empty YAML files)
	if config.Projects == nil {
		config.Projects = make(map[string]ProjectConfig)
	}

	// Validate and create Project instances
	projects := make(map[string]*Project)
	for name, projectConfig := range config.Projects {
		errors := ValidateProjectConfig(name, projectConfig)
		if len(errors) > 0 {
			return nil, nil, fmt.Errorf("invalid configuration for project '%s':\n%s",
				name, strings.Join(errors, "\n"))
		}

		// Apply defaults
		branch := projectConfig.Branch
		if branch == "" {
			branch = "main"
		}

		pullTimeout := projectConfig.PullTimeout
		if pullTimeout == 0 {
			pullTimeout = DefaultPullTimeout
		}

		postDeployTimeout := projectConfig.PostDeployTimeout
		if postDeployTimeout == 0 {
			postDeployTimeout = DefaultPostDeployTimeout
		}

		postDeploy := projectConfig.PostDeploy
		if postDeploy == nil {
			postDeploy = []interface{}{}
		}

		postActivateTimeout := projectConfig.PostActivateTimeout
		if postActivateTimeout == 0 {
			postActivateTimeout = DefaultPostActivateTimeout
		}

		postActivate := projectConfig.PostActivate
		if postActivate == nil {
			postActivate = []interface{}{}
		}

		// Resolve path to absolute
		resolvedPath, err := filepath.Abs(projectConfig.Path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve path for project '%s': %w", name, err)
		}

		// Resolve symlinks
		realPath, err := filepath.EvalSymlinks(resolvedPath)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to resolve symlinks for project '%s': %w", name, err)
		}

		projects[name] = &Project{
			Name:                name,
			Path:                realPath,
			Secret:              projectConfig.Secret,
			Branch:              branch,
			PullTimeout:         pullTimeout,
			PostDeployTimeout:   postDeployTimeout,
			PostDeploy:          postDeploy,
			PostActivateTimeout: postActivateTimeout,
			PostActivate:        postActivate,
		}
	}

	return &config, projects, nil
}

// ValidateProjectConfig validates a single project configuration
func ValidateProjectConfig(name string, config ProjectConfig) []string {
	var errors []string

	// Validate path
	if config.Path == "" {
		errors = append(errors, fmt.Sprintf("  - Project '%s': missing required 'path' field", name))
	} else {
		// Must be absolute
		if !filepath.IsAbs(config.Path) {
			errors = append(errors, fmt.Sprintf("  - Project '%s': path must be absolute, got '%s'", name, config.Path))
		}

		// Resolve symlinks to get real path
		resolvedPath, err := filepath.Abs(config.Path)
		if err != nil {
			errors = append(errors, fmt.Sprintf("  - Project '%s': cannot resolve path '%s': %v", name, config.Path, err))
		} else {
			realPath, err := filepath.EvalSymlinks(resolvedPath)
			if err != nil {
				errors = append(errors, fmt.Sprintf("  - Project '%s': cannot resolve path '%s': %v", name, config.Path, err))
			} else {
				// Check path exists
				info, err := os.Stat(realPath)
				if err != nil {
					if os.IsNotExist(err) {
						errors = append(errors, fmt.Sprintf("  - Project '%s': path does not exist: '%s'", name, realPath))
					} else {
						errors = append(errors, fmt.Sprintf("  - Project '%s': cannot stat path '%s': %v", name, realPath, err))
					}
				} else if !info.IsDir() {
					errors = append(errors, fmt.Sprintf("  - Project '%s': path is not a directory: '%s'", name, realPath))
				}

				// Check for zero-downtime deployment structure
				currentLink := filepath.Join(realPath, "current")
				sharedDir := filepath.Join(realPath, "shared")
				releasesDir := filepath.Join(realPath, "releases")

				// Verify current symlink exists
				if _, err := os.Lstat(currentLink); os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("  - Project '%s': missing 'current' symlink (run installer first): '%s'", name, realPath))
				} else {
					// Verify current points to a valid release with a git repository
					currentPath, err := filepath.EvalSymlinks(currentLink)
					if err != nil {
						errors = append(errors, fmt.Sprintf("  - Project '%s': 'current' symlink is broken: %v", name, err))
					} else {
						gitDir := filepath.Join(currentPath, ".git")
						if _, err := os.Stat(gitDir); os.IsNotExist(err) {
							errors = append(errors, fmt.Sprintf("  - Project '%s': current release is not a git repository (missing .git): '%s'", name, currentPath))
						}
					}
				}

				// Verify shared directory exists
				if _, err := os.Stat(sharedDir); os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("  - Project '%s': missing 'shared' directory (run installer first): '%s'", name, realPath))
				}

				// Verify releases directory exists
				if _, err := os.Stat(releasesDir); os.IsNotExist(err) {
					errors = append(errors, fmt.Sprintf("  - Project '%s': missing 'releases' directory (run installer first): '%s'", name, realPath))
				}

				// Check path is within allowed root if configured
				projectsRoot := os.Getenv("DEPLOBOX_PROJECTS_ROOT")
				if projectsRoot != "" {
					rootPath, err := filepath.EvalSymlinks(projectsRoot)
					if err == nil {
						relPath, err := filepath.Rel(rootPath, realPath)
						if err != nil || strings.HasPrefix(relPath, "..") {
							errors = append(errors, fmt.Sprintf("  - Project '%s': path '%s' is outside allowed root '%s'", name, realPath, rootPath))
						}
					}
				}
			}
		}
	}

	// Validate secret
	if config.Secret == "" {
		errors = append(errors, fmt.Sprintf("  - Project '%s': missing required 'secret' field", name))
	} else {
		if len(config.Secret) < MinSecretLength {
			errors = append(errors, fmt.Sprintf("  - Project '%s': secret too short (minimum %d characters)", name, MinSecretLength))
		}

		if ForbiddenSecrets[strings.ToLower(config.Secret)] {
			errors = append(errors, fmt.Sprintf("  - Project '%s': secret appears to be a placeholder value, replace with real secret", name))
		}
	}

	// Validate timeouts (must be positive if set, zero uses defaults)
	pullTimeout := config.PullTimeout
	if pullTimeout < 0 {
		errors = append(errors, fmt.Sprintf("  - Project '%s': pull_timeout must be a positive integer, got %d", name, pullTimeout))
	}

	postDeployTimeout := config.PostDeployTimeout
	if postDeployTimeout < 0 {
		errors = append(errors, fmt.Sprintf("  - Project '%s': post_deploy_timeout must be a positive integer, got %d", name, postDeployTimeout))
	}

	postActivateTimeout := config.PostActivateTimeout
	if postActivateTimeout < 0 {
		errors = append(errors, fmt.Sprintf("  - Project '%s': post_activate_timeout must be a positive integer, got %d", name, postActivateTimeout))
	}

	// Validate branch
	branch := config.Branch
	if branch == "" {
		branch = "main"
	}
	if strings.HasPrefix(branch, "-") {
		errors = append(errors, fmt.Sprintf("  - Project '%s': branch name cannot start with '-', got '%s'", name, branch))
	}

	// Validate post_deploy commands
	if config.PostDeploy != nil {
		for i, cmd := range config.PostDeploy {
			switch cmd.(type) {
			case string:
				// Valid
			case []interface{}:
				// Valid - list of commands
			default:
				errors = append(errors, fmt.Sprintf("  - Project '%s': post_deploy[%d] must be a string or list, got %T", name, i, cmd))
			}
		}
	}

	// Validate post_activate commands
	if config.PostActivate != nil {
		for i, cmd := range config.PostActivate {
			switch cmd.(type) {
			case string:
				// Valid
			case []interface{}:
				// Valid - list of commands
			default:
				errors = append(errors, fmt.Sprintf("  - Project '%s': post_activate[%d] must be a string or list, got %T", name, i, cmd))
			}
		}
	}

	return errors
}

// MatchesRef checks if a git ref matches the project's target branch
func (p *Project) MatchesRef(ref string) bool {
	return ref == fmt.Sprintf("refs/heads/%s", p.Branch)
}
