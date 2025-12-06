package main

import (
	"fmt"
	"os"
	"os/exec"
)

// installNginx configures nginx reverse proxy for deplobox
func installNginx(c *Config) error {
	if !hasSystemd {
		fmt.Printf("%-70s%s[OK]%s\n", "Skipping nginx setup (no systemd)...", colorGreen, colorReset)
		return nil
	}

	// Check if nginx is available
	if _, err := exec.LookPath("nginx"); err != nil {
		fmt.Printf("%-70s%s[WARN]%s\n", "Nginx not found, skipping configuration...", colorYellow, colorReset)
		return nil
	}

	sitePath := "/etc/nginx/sites-available/deplobox"
	enabledPath := "/etc/nginx/sites-enabled/deplobox"

	fmt.Printf("%-70s", "Creating nginx site config...")

	// Use webhook domain (where deplobox is hosted)
	domain := c.GetWebhookDomain()

	nginxConfig := fmt.Sprintf(`server {
    listen 80;
    server_name %s;

    location / {
        proxy_pass http://127.0.0.1:5000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Webhook-specific settings
        client_max_body_size 1m;
        proxy_read_timeout 60s;
    }
}
`, domain)

	if err := os.WriteFile(sitePath, []byte(nginxConfig), 0644); err != nil {
		fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
		return fmt.Errorf("writing nginx config: %w", err)
	}

	fmt.Printf("%s[OK]%s\n", colorGreen, colorReset)

	// Create symlink if it doesn't exist
	if _, err := os.Lstat(enabledPath); os.IsNotExist(err) {
		if err := runCmd("Linking nginx site config", "ln", "-sf", sitePath, enabledPath); err != nil {
			return err
		}
	} else {
		fmt.Printf("%-70s%s[OK]%s\n", "Nginx site already enabled...", colorGreen, colorReset)
	}

	if err := runCmd("Testing nginx configuration", "nginx", "-t"); err != nil {
		return err
	}

	return runCmd("Reloading nginx", "systemctl", "reload", "nginx")
}
