package dashboard

import "time"

// ── Constants ──

const (
	SiteTypeStatic = "static"
	SiteTypeProxy  = "proxy"

	SiteTypePocketbase = "pocketbase"

	SiteStatusActive = "active"
	SiteStatusPaused = "paused"

	SSLStatusNone    = "none"
	SSLStatusPending = "pending"
	SSLStatusActive  = "active"
	SSLStatusError   = "error"

	DBTypePocketbase = "pocketbase"

	PortRangeStart = 10000
	PortRangeEnd   = 20000

	NginxSitesAvailable = "/etc/nginx/sites-available"
	NginxSitesEnabled   = "/etc/nginx/sites-enabled"
	NginxDir            = "/etc/nginx"

	WebRootDir = "/var/www"

	DefaultAdminPort = 8090
)

// ── Models (mirrors Pocketbase collections) ──

type Site struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Domain        string `json:"domain"`
	Port          int    `json:"port"`
	RootDir       string `json:"root_dir"`
	SiteType      string `json:"site_type"` // "static" | "proxy" | "pocketbase"
	ProxyURL      string `json:"proxy_url,omitempty"`
	AdminEmail    string `json:"admin_email,omitempty"`
	AdminPassword string `json:"admin_password,omitempty"`
	SSLStatus     string `json:"ssl_status"`
	SSLExpiry     string `json:"ssl_expiry,omitempty"`
	Status        string `json:"status"`
	GitRepo       string `json:"git_repo,omitempty"`
	Notes         string `json:"notes,omitempty"`
	Created       string `json:"created"`
	Updated       string `json:"updated"`
}

type Database struct {
	ID            string `json:"id"`
	SiteID        string `json:"site_id"`
	Name          string `json:"name"`
	DBType        string `json:"db_type"`
	Port          int    `json:"port"`
	AdminEmail    string `json:"admin_email,omitempty"`
	AdminPassword string `json:"admin_password,omitempty"`
	Status        string `json:"status"`
	Created       string `json:"created"`
	Updated       string `json:"updated"`
}

// ── Request / Response helpers ──

type CreateSiteRequest struct {
	Name       string `json:"name"`
	Domain     string `json:"domain"`
	Port       int    `json:"port,omitempty"`
	SiteType   string `json:"site_type"`
	ProxyURL   string `json:"proxy_url,omitempty"`
	AdminEmail string `json:"admin_email,omitempty"`
	GitRepo    string `json:"git_repo,omitempty"`
	BuildCmd   string `json:"build_cmd,omitempty"`
	OutputDir  string `json:"output_dir,omitempty"`
	Notes      string `json:"notes,omitempty"`
}

type GitDeployRequest struct {
	BuildCmd  string `json:"build_cmd,omitempty"`
	OutputDir string `json:"output_dir,omitempty"`
}

type UpdateSiteRequest struct {
	Name          *string `json:"name,omitempty"`
	Domain        *string `json:"domain,omitempty"`
	Port          *int    `json:"port,omitempty"`
	SiteType      *string `json:"site_type,omitempty"`
	ProxyURL      *string `json:"proxy_url,omitempty"`
	AdminEmail    *string `json:"admin_email,omitempty"`
	AdminPassword *string `json:"admin_password,omitempty"`
	Status        *string `json:"status,omitempty"`
	Notes         *string `json:"notes,omitempty"`
}

type CreateDatabaseRequest struct {
	Name       string `json:"name"`
	AdminEmail string `json:"admin_email"`
}

type StatsResponse struct {
	TotalSites     int           `json:"total_sites"`
	ActiveSites    int           `json:"active_sites"`
	SSLActiveCount int           `json:"ssl_active_count"`
	TotalDatabases int           `json:"total_databases"`
	NGINXRunning   bool          `json:"nginx_running"`
	Metrics        SystemMetrics `json:"metrics"`
}

type NginxConfigInput struct {
	Domain     string
	Port       int
	RootDir    string
	SiteType   string
	ProxyURL   string
	SSLEntry   *SSLEntry
	ConfigPath string
}

type SSLEntry struct {
	CertPath string
	KeyPath  string
}

// utils

func Now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
