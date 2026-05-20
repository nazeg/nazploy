package dashboard

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// ── Sites ──

func HandleListSites(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	records, err := app.FindAllRecords("sites")
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, records)
}

func HandleCreateSite(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager, pm *PortManager) error {
	var req CreateSiteRequest
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.Name == "" || req.Domain == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "name and domain are required"})
	}

	// Determine port (either manual or auto-allocated)
	var port int
	var err error
	if req.Port != 0 {
		port = req.Port
	} else {
		port, err = pm.Next()
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
	}

	// Create Pocketbase record first to get a unique ID
	collection, err := app.FindCollectionByNameOrId("sites")
	if err != nil {
		if req.Port == 0 {
			pm.Release(port)
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "sites collection not found"})
	}

	record := core.NewRecord(collection)
	record.Id = generateRandomID()
	record.Set("name", req.Name)
	record.Set("domain", req.Domain)
	record.Set("port", port)
	record.Set("site_type", req.SiteType)
	record.Set("ssl_status", SSLStatusNone)
	record.Set("status", SiteStatusActive)
	record.Set("notes", req.Notes)

	// Create web root (site name slug + record ID)
	folderName := sanitizeSlug(req.Name) + "_" + record.Id
	rootDir, err := ngx.CreateWebRoot(folderName)
	if err != nil {
		if req.Port == 0 {
			pm.Release(port)
		}
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	record.Set("root_dir", rootDir)

	var backendPort int
	proxyURL := req.ProxyURL

	if req.SiteType == SiteTypeProxy && req.ProxyURL != "" {
		record.Set("proxy_url", req.ProxyURL)
	} else if req.SiteType == SiteTypePocketbase {
		if req.AdminEmail == "" {
			if req.Port == 0 {
				pm.Release(port)
			}
			os.RemoveAll(rootDir)
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "admin email is required for pocketbase site"})
		}
		// Allocate second port for PocketBase process
		bp, err := pm.Next()
		if err != nil {
			if req.Port == 0 {
				pm.Release(port)
			}
			os.RemoveAll(rootDir)
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to allocate pocketbase port: " + err.Error()})
		}
		backendPort = bp
		proxyURL = fmt.Sprintf("http://127.0.0.1:%d", backendPort)

		adminPass := generatePassword(24)
		record.Set("admin_email", req.AdminEmail)
		record.Set("admin_password", adminPass)
		record.Set("proxy_url", proxyURL)
	}

	if err := app.Save(record); err != nil {
		if req.Port == 0 {
			pm.Release(port)
		}
		if backendPort > 0 {
			pm.Release(backendPort)
		}
		os.RemoveAll(rootDir)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Generate and write Nginx config
	config, err := ngx.GenerateConfig(NginxConfigInput{
		Domain:   req.Domain,
		Port:     port,
		RootDir:  rootDir,
		SiteType: req.SiteType,
		ProxyURL: proxyURL,
	})
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "nginx config gen: " + err.Error()})
	}

	if err := ngx.WriteConfig(req.Domain, config); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "nginx config write: " + err.Error()})
	}

	// Start PocketBase instance if it's a pocketbase site
	if req.SiteType == SiteTypePocketbase {
		go launchPocketbaseInstance(record.Id, backendPort, req.AdminEmail, record.GetString("admin_password"))
	}

	// Reload Nginx
	if err := ngx.Reload(); err != nil {
		// Non-fatal: config is written, just nginx didn't reload
		return e.JSON(http.StatusOK, map[string]interface{}{
			"record":  record,
			"warning": "Nginx reload failed: " + err.Error(),
		})
	}

	return e.JSON(http.StatusCreated, record)
}

func HandleGetSite(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}
	return e.JSON(http.StatusOK, record)
}

func HandleUpdateSite(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	var req UpdateSiteRequest
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	oldDomain := record.GetString("domain")
	var domainChanged bool

	if req.Name != nil {
		record.Set("name", *req.Name)
	}
	if req.Domain != nil && *req.Domain != oldDomain {
		record.Set("domain", *req.Domain)
		domainChanged = true
	}
	if req.Port != nil {
		record.Set("port", *req.Port)
	}
	if req.SiteType != nil {
		record.Set("site_type", *req.SiteType)
	}
	if req.ProxyURL != nil {
		record.Set("proxy_url", *req.ProxyURL)
	}
	if req.Status != nil {
		record.Set("status", *req.Status)
	}
	if req.Notes != nil {
		record.Set("notes", *req.Notes)
	}

	if err := app.Save(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Clean up old domain config in Nginx if domain changed
	if domainChanged {
		ngx.RemoveConfig(oldDomain)
	}

	domain := record.GetString("domain")
	status := record.GetString("status")

	if status == SiteStatusPaused {
		// Pasif: Nginx symlink'ini kaldır ve pocketbase servisini durdur
		enabledPath := filepath.Join(NginxSitesEnabled, domain)
		os.Remove(enabledPath)
		if record.GetString("site_type") == SiteTypePocketbase {
			runCommand("systemctl", "stop", "pocketbase-"+record.Id)
		}
	} else {
		// Aktif: Pocketbase servisini başlat (gerekliyse) ve Nginx konfigürasyonunu yaz
		if record.GetString("site_type") == SiteTypePocketbase {
			runCommand("systemctl", "start", "pocketbase-"+record.Id)
		}

		var sslEntry *SSLEntry
		if record.GetString("ssl_status") == SSLStatusActive {
			sslEntry = &SSLEntry{
				CertPath: filepath.Join("/etc/letsencrypt/live", domain, "fullchain.pem"),
				KeyPath:  filepath.Join("/etc/letsencrypt/live", domain, "privkey.pem"),
			}
		}

		config, err := ngx.GenerateConfig(NginxConfigInput{
			Domain:   domain,
			Port:     record.GetInt("port"),
			RootDir:  record.GetString("root_dir"),
			SiteType: record.GetString("site_type"),
			ProxyURL: record.GetString("proxy_url"),
			SSLEntry: sslEntry,
		})
		if err != nil {
			return e.JSON(http.StatusOK, map[string]interface{}{
				"record":  record,
				"warning": "nginx config gen failed: " + err.Error(),
			})
		}

		if err := ngx.WriteConfig(domain, config); err != nil {
			return e.JSON(http.StatusOK, map[string]interface{}{
				"record":  record,
				"warning": "nginx config write failed: " + err.Error(),
			})
		}
	}

	ngx.Reload() // best effort

	return e.JSON(http.StatusOK, record)
}

func HandleDeleteSite(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	domain := record.GetString("domain")
	rootDir := record.GetString("root_dir")

	// Remove Nginx config
	ngx.RemoveConfig(domain)

	// Remove web root
	os.RemoveAll(rootDir)

	// If it's a PocketBase site, stop and remove the service
	if record.GetString("site_type") == SiteTypePocketbase {
		stopAndRemovePocketbaseService(record.Id)
		// Extract backend port from proxy_url
		backendPort := 0
		fmt.Sscanf(record.GetString("proxy_url"), "http://127.0.0.1:%d", &backendPort)
		if backendPort > 0 {
			killProcessOnPort(backendPort)
		}
		// Remove DB data dir
		dbDir := filepath.Join("/var/lib/dashboard/databases", record.Id)
		os.RemoveAll(dbDir)
	}

	// Delete associated databases
	databases, _ := app.FindAllRecords("databases", dbx.NewExp("site_id = {:id}", dbx.Params{"id": record.Id}))
	for _, db := range databases {
		// Kill the Pocketbase process if running
		stopAndRemovePocketbaseService(db.Id)
		dbPort := db.GetInt("port")
		if dbPort > 0 {
			killProcessOnPort(dbPort)
		}
		// Remove db data directory
		dbDir := filepath.Join("/var/lib/dashboard/databases", db.Id)
		os.RemoveAll(dbDir)
		app.Delete(db)
	}

	if err := app.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Reload Nginx
	ngx.Reload() // best effort

	return e.JSON(http.StatusOK, map[string]string{"message": "site deleted"})
}

// ── Deploy ──

func HandleDeploySite(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	if record.GetString("status") == SiteStatusPaused {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Pasif durumdaki siteler yayınlanamaz. Lütfen önce siteyi aktif hale getirin."})
	}

	domain := record.GetString("domain")

	// Regenerate and write config (in case files changed)
	var sslEntry *SSLEntry
	if record.GetString("ssl_status") == SSLStatusActive {
		sslEntry = &SSLEntry{
			CertPath: filepath.Join("/etc/letsencrypt/live", domain, "fullchain.pem"),
			KeyPath:  filepath.Join("/etc/letsencrypt/live", domain, "privkey.pem"),
		}
	}

	config, err := ngx.GenerateConfig(NginxConfigInput{
		Domain:   domain,
		Port:     record.GetInt("port"),
		RootDir:  record.GetString("root_dir"),
		SiteType: record.GetString("site_type"),
		ProxyURL: record.GetString("proxy_url"),
		SSLEntry: sslEntry,
	})
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err := ngx.WriteConfig(domain, config); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if err := ngx.Reload(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"message": "site deployed"})
}

// ── SSL ──

func HandleEnableSSL(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager, ssl *SSLManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	domain := record.GetString("domain")

	// Update status to pending
	record.Set("ssl_status", SSLStatusPending)
	app.Save(record)

	// Issue certificate asynchronously
	go func() {
		result, err := ssl.IssueCertificate(domain, record.GetInt("port"))
		if err != nil {
			record.Set("ssl_status", SSLStatusError)
			app.Save(record)
			return
		}

		record.Set("ssl_status", SSLStatusActive)
		record.Set("ssl_expiry", result.Expiry)
		app.Save(record)

		// Regenerate Nginx config with SSL
		config, err := ngx.GenerateConfig(NginxConfigInput{
			Domain:   domain,
			Port:     record.GetInt("port"),
			RootDir:  record.GetString("root_dir"),
			SiteType: record.GetString("site_type"),
			ProxyURL: record.GetString("proxy_url"),
			SSLEntry: &SSLEntry{CertPath: result.CertPath, KeyPath: result.KeyPath},
		})
		if err != nil {
			return
		}

		ngx.WriteConfig(domain, config)
		ngx.Reload()
	}()

	return e.JSON(http.StatusOK, map[string]string{"message": "SSL certificate request initiated", "ssl_status": SSLStatusPending})
}

func HandleDisableSSL(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	domain := record.GetString("domain")

	// Delete cert via certbot
	ssl := NewSSLManager()
	ssl.DeleteCertificate(domain)

	record.Set("ssl_status", SSLStatusNone)
	record.Set("ssl_expiry", "")
	app.Save(record)

	// Regenerate Nginx config without SSL
	config, err := ngx.GenerateConfig(NginxConfigInput{
		Domain:   domain,
		Port:     record.GetInt("port"),
		RootDir:  record.GetString("root_dir"),
		SiteType: record.GetString("site_type"),
		ProxyURL: record.GetString("proxy_url"),
	})
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	ngx.WriteConfig(domain, config)
	ngx.Reload()

	return e.JSON(http.StatusOK, map[string]string{"message": "SSL disabled"})
}

func HandleSSLStatus(e *core.RequestEvent, app *pocketbase.PocketBase, ssl *SSLManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	domain := record.GetString("domain")
	certInfo, err := ssl.GetCertificateStatus(domain)
	if err != nil || certInfo == nil {
		return e.JSON(http.StatusOK, map[string]interface{}{
			"ssl_status": record.GetString("ssl_status"),
			"expiry":     record.GetString("ssl_expiry"),
		})
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"ssl_status": record.GetString("ssl_status"),
		"expiry":     certInfo.Expiry,
		"cert_path":  certInfo.CertPath,
	})
}

// ── Databases (per-site Pocketbase instances) ──

func HandleCreateDatabase(e *core.RequestEvent, app *pocketbase.PocketBase, pm *PortManager) error {
	siteID := e.Request.PathValue("id")

	// Verify site exists
	if _, err := app.FindRecordById("sites", siteID); err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	var req CreateDatabaseRequest
	if err := e.BindBody(&req); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.Name == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "database name is required"})
	}

	// Allocate port
	port, err := pm.Next()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Create the collection record
	collection, err := app.FindCollectionByNameOrId("databases")
	if err != nil {
		pm.Release(port)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "databases collection not found"})
	}

	record := core.NewRecord(collection)
	record.Set("site_id", siteID)
	record.Set("name", req.Name)
	record.Set("db_type", DBTypePocketbase)
	record.Set("port", port)
	record.Set("admin_email", req.AdminEmail)
	record.Set("status", SiteStatusActive)

	// Generate random admin password
	adminPass := generatePassword(24)
	record.Set("admin_password", adminPass)

	if err := app.Save(record); err != nil {
		pm.Release(port)
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Launch a new Pocketbase instance for this site
	go launchPocketbaseInstance(record.Id, port, req.AdminEmail, adminPass)

	return e.JSON(http.StatusCreated, record)
}

func HandleListDatabases(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	siteID := e.Request.PathValue("id")

	records, err := app.FindAllRecords("databases",
		dbx.NewExp("site_id = {:id}", dbx.Params{"id": siteID}),
	)
	if err != nil {
		return e.JSON(http.StatusOK, []interface{}{})
	}

	return e.JSON(http.StatusOK, records)
}

func HandleDeleteDatabase(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	dbID := e.Request.PathValue("dbId")

	record, err := app.FindRecordById("databases", dbID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "database not found"})
	}

	// Kill the process on that port
	port := record.GetInt("port")
	if port > 0 {
		killProcessOnPort(port)
	}

	// Remove data directory
	dbDir := filepath.Join("/var/lib/dashboard/databases", dbID)
	os.RemoveAll(dbDir)

	if err := app.Delete(record); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"message": "database deleted"})
}

// ── Ports ──

func HandleNextPort(e *core.RequestEvent, pm *PortManager) error {
	port, err := pm.Next()
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]int{"port": port})
}

// ── Nginx ──

func HandleNginxStatus(e *core.RequestEvent, ngx *NginxManager) error {
	running := ngx.IsRunning()
	return e.JSON(http.StatusOK, map[string]interface{}{
		"running": running,
	})
}

func HandleNginxReload(e *core.RequestEvent, ngx *NginxManager) error {
	if err := ngx.Reload(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]string{"message": "nginx reloaded"})
}

// ── Stats ──

func HandleStats(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	sites, _ := app.FindAllRecords("sites")
	databases, _ := app.FindAllRecords("databases")

	totalSites := len(sites)
	activeSites := 0
	sslCount := 0
	for _, s := range sites {
		if s.GetString("status") == SiteStatusActive {
			activeSites++
		}
		if s.GetString("ssl_status") == SSLStatusActive {
			sslCount++
		}
	}

	ngx := NewNginxManager()

	stats := StatsResponse{
		TotalSites:     totalSites,
		ActiveSites:    activeSites,
		SSLActiveCount: sslCount,
		TotalDatabases: len(databases),
		NGINXRunning:   ngx.IsRunning(),
		Metrics:        GetSystemMetrics(),
	}

	return e.JSON(http.StatusOK, stats)
}

// ── Helpers ──

func generatePassword(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			// fallback to simple char
			b[i] = charset[i%len(charset)]
			continue
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

func launchPocketbaseInstance(id string, port int, adminEmail, adminPassword string) {
	startPocketbaseService(id, port, adminEmail, adminPassword)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func startPocketbaseService(id string, port int, adminEmail, adminPassword string) error {
	if runtime.GOOS == "windows" {
		dataDir := filepath.Join("d:\\WEB\\nazploydashboard2\\dashboard\\databases", id)
		os.MkdirAll(dataDir, 0755)

		executable, err := os.Executable()
		if err != nil {
			executable = "pocketbase"
		}

		cmd := exec.Command(executable, "serve", "--dir="+dataDir, fmt.Sprintf("--http=0.0.0.0:%d", port))
		err = cmd.Start()
		if err != nil {
			log.Printf("Failed to start pocketbase locally: %v", err)
			return err
		}

		go func() {
			time.Sleep(2 * time.Second)
			exec.Command(executable, "admin", "create", adminEmail, adminPassword, "--dir="+dataDir).Run()
		}()
		return nil
	}

	dataDir := fmt.Sprintf("/var/lib/dashboard/databases/%s", id)
	os.MkdirAll(dataDir, 0755)

	executable := "/root/dashboard/pocketbase_bin"
	if _, err := os.Stat(executable); os.IsNotExist(err) {
		var errExe error
		executable, errExe = os.Executable()
		if errExe != nil {
			executable = "pocketbase"
		}
	}

	serviceName := fmt.Sprintf("pocketbase-%s", id)
	serviceFileContent := fmt.Sprintf(`[Unit]
Description=PocketBase Service for %s
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=%s
ExecStart=%s serve --dir=%s --http=0.0.0.0:%d
Restart=always

[Install]
WantedBy=multi-user.target
`, id, dataDir, executable, dataDir, port)

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if err := os.WriteFile(servicePath, []byte(serviceFileContent), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	runCommand("systemctl", "daemon-reload")
	runCommand("systemctl", "enable", serviceName)
	runCommand("systemctl", "start", serviceName)

	go func() {
		time.Sleep(3 * time.Second)
		exec.Command(executable, "admin", "create", adminEmail, adminPassword, "--dir="+dataDir).Run()
	}()

	return nil
}

func stopAndRemovePocketbaseService(id string) {
	if runtime.GOOS == "windows" {
		return
	}
	serviceName := fmt.Sprintf("pocketbase-%s", id)
	runCommand("systemctl", "stop", serviceName)
	runCommand("systemctl", "disable", serviceName)

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	os.Remove(servicePath)
	runCommand("systemctl", "daemon-reload")
}

func killProcessOnPort(port int) {
	if runtime.GOOS == "windows" {
		cmdStr := fmt.Sprintf("Stop-Process -Id (Get-NetTCPConnection -LocalPort %d).OwningProcess -Force", port)
		exec.Command("powershell", "-Command", cmdStr).Run()
		return
	}
	exec.Command("fuser", "-k", fmt.Sprintf("%d/tcp", port)).Run()
}

func sanitizeSlug(s string) string {
	s = strings.ToLower(s)
	// Replace non-alphanumeric with hyphen
	reg := regexp.MustCompile("[^a-z0-9]+")
	s = reg.ReplaceAllString(s, "-")
	// Trim hyphens
	s = strings.Trim(s, "-")
	if s == "" {
		return "site"
	}
	return s
}

func generateRandomID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 15)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			b[i] = charset[i%len(charset)]
			continue
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
