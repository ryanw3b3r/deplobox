package templates

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// Template names
const (
	NginxSite          = "nginx-site"
	NginxLaravelSite   = "nginx-laravel-site"
	SystemdService     = "systemd-service"
)

// TemplateData holds variables for template rendering.
type TemplateData map[string]string

// GetTemplatePaths returns the search paths for templates
func GetTemplatePaths(templateName string) []string {
	filename := templateName + ".template"
	return []string{
		filepath.Join(".", "templates", filename),
		filepath.Join(".", "config", "templates", filename),
		filepath.Join("/etc", "deplobox", "templates", filename),
	}
}

// GetTemplate returns the raw template content by name.
// Templates are loaded from the filesystem in the following order:
// 1. ./templates/<name>.template
// 2. ./config/templates/<name>.template
// 3. /etc/deplobox/templates/<name>.template
func GetTemplate(name string) (string, error) {
	// Validate template name
	if !ValidateTemplate(name) {
		return "", fmt.Errorf("unknown template: %s", name)
	}

	// Try to find template file
	paths := GetTemplatePaths(name)
	for _, path := range paths {
		if content, err := os.ReadFile(path); err == nil {
			return string(content), nil
		}
	}

	return "", fmt.Errorf("template file not found: %s (searched: %v)", name, paths)
}

// Render renders a template with the given data.
// Uses {{PLACEHOLDER}} syntax for variable substitution.
//
// Example:
//   data := TemplateData{
//       "DOMAIN": "example.com",
//       "USER": "deploybot",
//   }
//   rendered, err := Render(NginxSite, data)
func Render(templateName string, data TemplateData) (string, error) {
	tmplContent, err := GetTemplate(templateName)
	if err != nil {
		return "", err
	}

	// Replace placeholders
	rendered := tmplContent
	for key, value := range data {
		placeholder := fmt.Sprintf("{{%s}}", key)
		rendered = strings.ReplaceAll(rendered, placeholder, value)
	}

	return rendered, nil
}

// RenderWithGoTemplate renders a template using Go's text/template package.
// This allows for more complex template logic (loops, conditionals, etc.).
// The data must be a struct or map that can be used with Go templates.
func RenderWithGoTemplate(templateName string, data interface{}) (string, error) {
	tmplContent, err := GetTemplate(templateName)
	if err != nil {
		return "", err
	}

	// Parse template
	tmpl, err := template.New(templateName).Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return buf.String(), nil
}

// RenderNginxSite renders the nginx site template.
func RenderNginxSite(domain string) (string, error) {
	return Render(NginxSite, TemplateData{
		"DOMAIN": domain,
	})
}

// RenderSystemdService renders the systemd service template.
func RenderSystemdService(user, group, workingDir, deploboxHome, logFile, dbPath string) (string, error) {
	return Render(SystemdService, TemplateData{
		"USER":         user,
		"GROUP":        group,
		"WORKING_DIR":  workingDir,
		"DEPLOBOX_HOME": deploboxHome,
		"LOG_FILE":     logFile,
		"DB_PATH":      dbPath,
	})
}

// ListTemplates returns a list of all available template names.
func ListTemplates() []string {
	return []string{
		NginxSite,
		NginxLaravelSite,
		SystemdService,
	}
}

// ValidateTemplate checks if a template name is valid.
func ValidateTemplate(name string) bool {
	validNames := map[string]bool{
		NginxSite:        true,
		NginxLaravelSite: true,
		SystemdService:   true,
	}
	return validNames[name]
}
