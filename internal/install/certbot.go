package install

import (
	"os/exec"
)

// setupCertbot requests Let's Encrypt SSL certificate
func setupCertbot(c *Config) error {
	// Skip if no email provided
	if c.CertbotEmail == "" {
		printSuccess("Skipping certbot (no EMAIL set)...")
		return nil
	}

	// Skip if no systemd or nginx
	if !hasSystemd {
		printSuccess("Skipping certbot (no systemd)...")
		return nil
	}

	if _, err := exec.LookPath("nginx"); err != nil {
		printSuccess("Skipping certbot (nginx not available)...")
		return nil
	}

	if _, err := exec.LookPath("certbot"); err != nil {
		printWarn("Certbot not found, skipping SSL setup...")
		return nil
	}

	return runCmd(
		"Requesting Let's Encrypt certificate for "+c.ProjectDomain,
		"certbot",
		"--nginx",
		"--non-interactive",
		"--agree-tos",
		"--redirect",
		"--email", c.CertbotEmail,
		"-d", c.ProjectDomain,
	)
}
