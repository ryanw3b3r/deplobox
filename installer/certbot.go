package main

import (
	"fmt"
	"os/exec"
)

// setupCertbot requests Let's Encrypt SSL certificate
func setupCertbot(c *Config) error {
	// Skip if no email provided
	if c.CertbotEmail == "" {
		fmt.Printf("%-70s%s[OK]%s\n", "Skipping certbot (no EMAIL set)...", colorGreen, colorReset)
		return nil
	}

	// Skip if no systemd or nginx
	if !hasSystemd {
		fmt.Printf("%-70s%s[OK]%s\n", "Skipping certbot (no systemd)...", colorGreen, colorReset)
		return nil
	}

	if _, err := exec.LookPath("nginx"); err != nil {
		fmt.Printf("%-70s%s[OK]%s\n", "Skipping certbot (nginx not available)...", colorGreen, colorReset)
		return nil
	}

	if _, err := exec.LookPath("certbot"); err != nil {
		fmt.Printf("%-70s%s[WARN]%s\n", "Certbot not found, skipping SSL setup...", colorYellow, colorReset)
		return nil
	}

	// Use webhook domain for SSL
	domain := c.GetWebhookDomain()

	return runCmd(
		"Requesting Let's Encrypt certificate",
		"certbot",
		"--nginx",
		"--non-interactive",
		"--agree-tos",
		"--redirect",
		"--email", c.CertbotEmail,
		"-d", domain,
	)
}
