package dashboard

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// SSLManager handles Let's Encrypt certificate via Certbot
type SSLManager struct {
	CertbotPath string
	SSLBaseDir  string
}

func NewSSLManager() *SSLManager {
	return &SSLManager{
		CertbotPath: "/usr/bin/certbot",
		SSLBaseDir:  "/etc/letsencrypt/live",
	}
}

type CertResult struct {
	CertPath string
	KeyPath  string
	Expiry   string
}

// IssueCertificate obtains a Let's Encrypt SSL certificate for the given domain
func (m *SSLManager) IssueCertificate(domain string, port int) (*CertResult, error) {
	// Check if certbot exists
	if _, err := os.Stat(m.CertbotPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("certbot not found at %s (install with: apt install certbot python3-certbot-nginx)", m.CertbotPath)
	}

	// Use the webroot or standalone mode
	// For simplicity, we use standalone mode (requires port 80/443 free)
	// In production, you might want to use webroot mode with existing nginx
	rootDir := filepath.Join(WebRootDir, domain)
	_ = os.MkdirAll(rootDir, 0755)

	// Run certbot in webroot mode
	cmd := exec.Command(m.CertbotPath, "certonly", "--webroot",
		"-w", rootDir,
		"-d", domain,
		"--non-interactive",
		"--agree-tos",
		"--email", fmt.Sprintf("admin@%s", domain),
		"--keep-until-expiring",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("certbot failed: %w\nOutput: %s", err, string(output))
	}

	// Build cert paths
	certPath := filepath.Join(m.SSLBaseDir, domain, "fullchain.pem")
	keyPath := filepath.Join(m.SSLBaseDir, domain, "privkey.pem")

	// Get expiry date
	expiry, _ := m.getCertificateExpiry(certPath)

	return &CertResult{
		CertPath: certPath,
		KeyPath:  keyPath,
		Expiry:   expiry,
	}, nil
}

// RenewCertificate attempts to renew a certificate
func (m *SSLManager) RenewCertificate(domain string) (*CertResult, error) {
	cmd := exec.Command(m.CertbotPath, "renew", "--cert-name", domain, "--non-interactive")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("certbot renew failed: %w\nOutput: %s", err, string(output))
	}

	certPath := filepath.Join(m.SSLBaseDir, domain, "fullchain.pem")
	keyPath := filepath.Join(m.SSLBaseDir, domain, "privkey.pem")
	expiry, _ := m.getCertificateExpiry(certPath)

	return &CertResult{
		CertPath: certPath,
		KeyPath:  keyPath,
		Expiry:   expiry,
	}, nil
}

// GetCertificateStatus returns the expiry date if certificate exists
func (m *SSLManager) GetCertificateStatus(domain string) (*CertResult, error) {
	certPath := filepath.Join(m.SSLBaseDir, domain, "fullchain.pem")

	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		return nil, nil // no certificate
	}

	keyPath := filepath.Join(m.SSLBaseDir, domain, "privkey.pem")
	expiry, err := m.getCertificateExpiry(certPath)
	if err != nil {
		return &CertResult{CertPath: certPath, KeyPath: keyPath}, nil
	}

	return &CertResult{
		CertPath: certPath,
		KeyPath:  keyPath,
		Expiry:   expiry,
	}, nil
}

// DeleteCertificate removes the certificate for a domain
func (m *SSLManager) DeleteCertificate(domain string) error {
	cmd := exec.Command(m.CertbotPath, "delete", "--cert-name", domain, "--non-interactive")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certbot delete failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// getCertificateExpiry extracts the expiry date from a certificate file using openssl
func (m *SSLManager) getCertificateExpiry(certPath string) (string, error) {
	cmd := exec.Command("openssl", "x509", "-enddate", "-noout", "-in", certPath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("openssl failed: %w", err)
	}

	// Output: notAfter=Apr 12 12:00:00 2025 GMT
	parts := strings.SplitN(strings.TrimSpace(string(output)), "=", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("unexpected openssl output: %s", string(output))
	}

	expiryTime, err := time.Parse("Jan 2 15:04:05 2006 MST", parts[1])
	if err != nil {
		return "", fmt.Errorf("parse expiry: %w", err)
	}

	return expiryTime.Format(time.RFC3339), nil
}
