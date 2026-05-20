package dashboard

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"
)

// NginxManager manages Nginx site configurations
type NginxManager struct{}

func NewNginxManager() *NginxManager {
	return &NginxManager{}
}

// ── Nginx config templates ──

const nginxStaticSite = `server {
    listen {{.Port}};
    listen [::]:{{.Port}};

    server_name {{.Domain}};

    root {{.RootDir}};
    index index.html index.htm index.php;

    location / {
        try_files $uri $uri/ =404;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php8.1-fpm.sock;
    }

    access_log /var/log/nginx/{{.Domain}}-access.log;
    error_log  /var/log/nginx/{{.Domain}}-error.log;
}
`

const nginxStaticSiteSSL = `server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;

    server_name {{.Domain}};

    ssl_certificate     {{.SSLEntry.CertPath}};
    ssl_certificate_key {{.SSLEntry.KeyPath}};

    root {{.RootDir}};
    index index.html index.htm index.php;

    location / {
        try_files $uri $uri/ =404;
    }

    location ~ \.php$ {
        include snippets/fastcgi-php.conf;
        fastcgi_pass unix:/var/run/php/php8.1-fpm.sock;
    }

    access_log /var/log/nginx/{{.Domain}}-access.log;
    error_log  /var/log/nginx/{{.Domain}}-error.log;
}

# HTTP → HTTPS redirect
server {
    listen {{.Port}};
    listen [::]:{{.Port}};

    server_name {{.Domain}};

    return 301 https://$server_name$request_uri;
}
`

const nginxProxySite = `server {
    listen {{.Port}};
    listen [::]:{{.Port}};

    server_name {{.Domain}};

    location / {
        proxy_pass {{.ProxyURL}};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    access_log /var/log/nginx/{{.Domain}}-access.log;
    error_log  /var/log/nginx/{{.Domain}}-error.log;
}
`

const nginxProxySiteSSL = `server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;

    server_name {{.Domain}};

    ssl_certificate     {{.SSLEntry.CertPath}};
    ssl_certificate_key {{.SSLEntry.KeyPath}};

    location / {
        proxy_pass {{.ProxyURL}};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    access_log /var/log/nginx/{{.Domain}}-access.log;
    error_log  /var/log/nginx/{{.Domain}}-error.log;
}

# HTTP → HTTPS redirect
server {
    listen {{.Port}};
    listen [::]:{{.Port}};

    server_name {{.Domain}};

    return 301 https://$server_name$request_uri;
}
`

const nginxPocketbaseSite = `server {
    listen {{.Port}};
    listen [::]:{{.Port}};

    server_name {{.Domain}};

    root {{.RootDir}};
    index index.html index.htm;

    location ~ ^/(api|_) {
        proxy_pass {{.ProxyURL}};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    location / {
        try_files $uri $uri/ /index.html =404;
    }

    access_log /var/log/nginx/{{.Domain}}-access.log;
    error_log  /var/log/nginx/{{.Domain}}-error.log;
}
`

const nginxPocketbaseSiteSSL = `server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;

    server_name {{.Domain}};

    ssl_certificate     {{.SSLEntry.CertPath}};
    ssl_certificate_key {{.SSLEntry.KeyPath}};

    root {{.RootDir}};
    index index.html index.htm;

    location ~ ^/(api|_) {
        proxy_pass {{.ProxyURL}};
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";

        proxy_read_timeout 86400s;
        proxy_send_timeout 86400s;
    }

    location / {
        try_files $uri $uri/ /index.html =404;
    }

    access_log /var/log/nginx/{{.Domain}}-access.log;
    error_log  /var/log/nginx/{{.Domain}}-error.log;
}

# HTTP → HTTPS redirect
server {
    listen {{.Port}};
    listen [::]:{{.Port}};

    server_name {{.Domain}};

    return 301 https://$server_name$request_uri;
}
`

func (m *NginxManager) GenerateConfig(in NginxConfigInput) (string, error) {
	var tmplStr string

	if in.SSLEntry != nil {
		tmplStr = nginxStaticSiteSSL
		if in.SiteType == SiteTypeProxy {
			tmplStr = nginxProxySiteSSL
		} else if in.SiteType == SiteTypePocketbase {
			tmplStr = nginxPocketbaseSiteSSL
		}
	} else {
		tmplStr = nginxStaticSite
		if in.SiteType == SiteTypeProxy {
			tmplStr = nginxProxySite
		} else if in.SiteType == SiteTypePocketbase {
			tmplStr = nginxPocketbaseSite
		}
	}

	// Default root dir
	if in.RootDir == "" {
		in.RootDir = filepath.Join(WebRootDir, in.Domain)
	}

	if in.Port == 0 {
		in.Port = 80
	}

	tmpl, err := template.New("nginx").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("template parse: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, in); err != nil {
		return "", fmt.Errorf("template execute: %w", err)
	}

	return buf.String(), nil
}

func (m *NginxManager) WriteConfig(domain, config string) error {
	// Ensure sites-available directory exists
	if err := os.MkdirAll(NginxSitesAvailable, 0755); err != nil {
		return fmt.Errorf("mkdir sites-available: %w", err)
	}

	configPath := filepath.Join(NginxSitesAvailable, domain)
	if err := os.WriteFile(configPath, []byte(config), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	// Create symlink in sites-enabled
	enabledPath := filepath.Join(NginxSitesEnabled, domain)
	// Remove old symlink if exists
	os.Remove(enabledPath)
	if err := os.Symlink(configPath, enabledPath); err != nil {
		return fmt.Errorf("symlink: %w", err)
	}

	return nil
}

func (m *NginxManager) RemoveConfig(domain string) error {
	configPath := filepath.Join(NginxSitesAvailable, domain)
	enabledPath := filepath.Join(NginxSitesEnabled, domain)

	os.Remove(configPath)
	os.Remove(enabledPath)

	return nil
}

func (m *NginxManager) Reload() error {
	// Test config first
	if err := exec.Command("nginx", "-t").Run(); err != nil {
		return fmt.Errorf("nginx config test failed: %w", err)
	}

	if err := exec.Command("nginx", "-s", "reload").Run(); err != nil {
		return fmt.Errorf("nginx reload: %w", err)
	}

	return nil
}

func (m *NginxManager) IsRunning() bool {
	err := exec.Command("nginx", "-v").Run()
	return err == nil
}

func (m *NginxManager) CreateWebRoot(domain string) (string, error) {
	rootDir := filepath.Join(WebRootDir, domain)
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return "", fmt.Errorf("create web root: %w", err)
	}

	// Create a default index.html
	indexHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body { font-family: system-ui, sans-serif; display: flex; align-items: center; justify-content: center; height: 100vh; margin: 0; background: #f5f5f5; }
        .card { text-align: center; padding: 2rem; background: white; border-radius: 12px; box-shadow: 0 2px 8px rgba(0,0,0,0.1); }
        h1 { color: #333; }
        p { color: #666; }
    </style>
</head>
<body>
    <div class="card">
        <h1>%s</h1>
        <p>Site is active. Deploy your content to %s</p>
    </div>
</body>
</html>`, domain, domain, rootDir)

	if err := os.WriteFile(filepath.Join(rootDir, "index.html"), []byte(indexHTML), 0644); err != nil {
		return rootDir, fmt.Errorf("create index.html: %w", err)
	}

	return rootDir, nil
}
