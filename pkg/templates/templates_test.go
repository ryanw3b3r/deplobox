package templates

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestTemplates creates temporary template files for testing
func setupTestTemplates(t *testing.T) func() {
	tmpDir := t.TempDir()
	templatesDir := filepath.Join(tmpDir, "templates")
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		t.Fatalf("Failed to create templates directory: %v", err)
	}

	// Create nginx-site.template
	nginxContent := `server {
    listen 80;
    server_name {{DOMAIN}};

    location / {
        proxy_pass http://localhost:3000;
    }
}`
	if err := os.WriteFile(filepath.Join(templatesDir, "nginx-site.template"), []byte(nginxContent), 0644); err != nil {
		t.Fatalf("Failed to create nginx-site.template: %v", err)
	}

	// Create nginx-laravel-site.template
	if err := os.WriteFile(filepath.Join(templatesDir, "nginx-laravel-site.template"), []byte(nginxContent), 0644); err != nil {
		t.Fatalf("Failed to create nginx-laravel-site.template: %v", err)
	}

	// Create systemd-service.template
	systemdContent := `[Unit]
Description=Deplobox Service

[Service]
User={{USER}}
Group={{GROUP}}
WorkingDirectory={{WORKING_DIR}}

[Install]
WantedBy=multi-user.target`
	if err := os.WriteFile(filepath.Join(templatesDir, "systemd-service.template"), []byte(systemdContent), 0644); err != nil {
		t.Fatalf("Failed to create systemd-service.template: %v", err)
	}

	// Change to temp directory so relative paths work
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)

	// Return cleanup function
	return func() {
		os.Chdir(oldWd)
	}
}

func TestGetTemplate(t *testing.T) {
	cleanup := setupTestTemplates(t)
	defer cleanup()

	tests := []struct {
		name        string
		templateName string
		wantErr     bool
		contains    string
	}{
		{
			"nginx site template",
			NginxSite,
			false,
			"server_name",
		},
		{
			"nginx laravel template",
			NginxLaravelSite,
			false,
			"server_name",
		},
		{
			"systemd service template",
			SystemdService,
			false,
			"[Unit]",
		},
		{
			"unknown template",
			"invalid-template",
			true,
			"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetTemplate(tt.templateName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetTemplate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !strings.Contains(got, tt.contains) {
				t.Errorf("GetTemplate() should contain %q", tt.contains)
			}
		})
	}
}

func TestRender(t *testing.T) {
	cleanup := setupTestTemplates(t)
	defer cleanup()

	tests := []struct {
		name         string
		templateName string
		data         TemplateData
		wantContains string
		wantErr      bool
	}{
		{
			"render nginx with domain",
			NginxSite,
			TemplateData{"DOMAIN": "example.com"},
			"server_name example.com",
			false,
		},
		{
			"render systemd service",
			SystemdService,
			TemplateData{
				"USER":         "deploybot",
				"GROUP":        "www-data",
				"WORKING_DIR":  "/home/deploybot",
				"DEPLOBOX_HOME": "/home/deploybot/deplobox",
				"LOG_FILE":     "/var/log/deplobox.log",
				"DB_PATH":      "/var/lib/deplobox.db",
			},
			"User=deploybot",
			false,
		},
		{
			"unknown template",
			"invalid",
			TemplateData{},
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Render(tt.templateName, tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("Render() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !strings.Contains(got, tt.wantContains) {
				t.Errorf("Render() should contain %q, got: %s", tt.wantContains, got)
			}
		})
	}
}

func TestRenderNginxSite(t *testing.T) {
	cleanup := setupTestTemplates(t)
	defer cleanup()

	domain := "example.com"
	rendered, err := RenderNginxSite(domain)
	if err != nil {
		t.Fatalf("RenderNginxSite() error = %v", err)
	}

	if !strings.Contains(rendered, domain) {
		t.Errorf("RenderNginxSite() should contain domain %q", domain)
	}

	if !strings.Contains(rendered, "server_name") {
		t.Error("RenderNginxSite() should contain 'server_name'")
	}
}

func TestRenderSystemdService(t *testing.T) {
	cleanup := setupTestTemplates(t)
	defer cleanup()

	rendered, err := RenderSystemdService(
		"deploybot",
		"www-data",
		"/home/deploybot",
		"/home/deploybot/deplobox",
		"/var/log/deplobox.log",
		"/var/lib/deplobox.db",
	)

	if err != nil {
		t.Fatalf("RenderSystemdService() error = %v", err)
	}

	expectations := []string{
		"User=deploybot",
		"Group=www-data",
		"WorkingDirectory=/home/deploybot",
	}

	for _, expected := range expectations {
		if !strings.Contains(rendered, expected) {
			t.Errorf("RenderSystemdService() should contain %q", expected)
		}
	}
}

func TestListTemplates(t *testing.T) {
	templates := ListTemplates()

	if len(templates) != 3 {
		t.Errorf("ListTemplates() returned %d templates, want 3", len(templates))
	}

	// Check all template names are present
	expectedNames := map[string]bool{
		NginxSite:        false,
		NginxLaravelSite: false,
		SystemdService:   false,
	}

	for _, name := range templates {
		if _, exists := expectedNames[name]; exists {
			expectedNames[name] = true
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("ListTemplates() missing template: %s", name)
		}
	}
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		want         bool
	}{
		{"valid nginx site", NginxSite, true},
		{"valid systemd service", SystemdService, true},
		{"invalid template", "invalid-template", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateTemplate(tt.templateName)
			if got != tt.want {
				t.Errorf("ValidateTemplate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRenderWithGoTemplate(t *testing.T) {
	// Note: Current templates use {{PLACEHOLDER}} syntax (simple replacement),
	// not Go template syntax, so RenderWithGoTemplate will fail to parse them
	data := struct {
		Domain string
	}{
		Domain: "example.com",
	}

	// This should fail because template uses {{DOMAIN}} (which Go templates see as a function)
	// rather than {{.Domain}} (proper Go template syntax)
	_, err := RenderWithGoTemplate(NginxSite, data)
	if err == nil {
		t.Error("RenderWithGoTemplate() should fail with current template syntax")
	}

	// Test with unknown template
	_, err = RenderWithGoTemplate("invalid", data)
	if err == nil {
		t.Error("RenderWithGoTemplate() should fail with unknown template")
	}
}

// Benchmark tests

func BenchmarkGetTemplate(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = GetTemplate(NginxSite)
	}
}

func BenchmarkRender(b *testing.B) {
	data := TemplateData{"DOMAIN": "example.com"}

	for i := 0; i < b.N; i++ {
		_, _ = Render(NginxSite, data)
	}
}

func BenchmarkRenderSystemdService(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = RenderSystemdService(
			"deploybot",
			"www-data",
			"/home/deploybot",
			"/home/deploybot/deplobox",
			"/var/log/deplobox.log",
			"/var/lib/deplobox.db",
		)
	}
}
