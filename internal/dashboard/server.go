package dashboard

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
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
		if req.Port < PortRangeStart || req.Port > PortRangeEnd {
			return e.JSON(http.StatusBadRequest, map[string]string{
				"error": fmt.Sprintf("port must be between %d and %d", PortRangeStart, PortRangeEnd),
			})
		}
		port = req.Port
	} else {
		port, err = pm.Next(app)
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
	if req.GitRepo != "" {
		record.Set("git_repo", req.GitRepo)
		record.Set("git_branch", req.GitBranch)
		record.Set("build_cmd", req.BuildCmd)
		record.Set("output_dir", req.OutputDir)
		record.Set("git_status", "idle")
	}

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
		bp, err := pm.Next(app)
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
		SiteID:   record.Id,
	})
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "nginx config gen: " + err.Error()})
	}

	// Clean up old-style domain config if it exists
	ngx.RemoveConfig(req.Domain)

	if err := ngx.WriteConfig(record.Id, config); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "nginx config write: " + err.Error()})
	}

	// Start PocketBase instance if it's a pocketbase site
	if req.SiteType == SiteTypePocketbase {
		go launchPocketbaseInstance(record.Id, backendPort, req.AdminEmail, record.GetString("admin_password"), true)
	}

	// If git_repo is provided, clone and build in background
	if req.GitRepo != "" {
		go func() {
			if err := CloneAndBuild(app, record.Id); err != nil {
				log.Printf("[GitDeploy] Hata (site: %s): %v", record.Id, err)
			} else {
				ngx.Reload() // reload nginx after files are in place
			}
		}()
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
	if req.GitRepo != nil {
		record.Set("git_repo", *req.GitRepo)
	}
	if req.GitBranch != nil {
		record.Set("git_branch", *req.GitBranch)
	}
	if req.BuildCmd != nil {
		record.Set("build_cmd", *req.BuildCmd)
	}
	if req.OutputDir != nil {
		record.Set("output_dir", *req.OutputDir)
	}
	if req.GitStatus != nil {
		record.Set("git_status", *req.GitStatus)
	}
	if req.GitLog != nil {
		record.Set("git_log", *req.GitLog)
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
		enabledPath := filepath.Join(NginxSitesEnabled, record.Id)
		os.Remove(enabledPath)
		// Clean up old-style domain symlink if it exists
		os.Remove(filepath.Join(NginxSitesEnabled, domain))
		if record.GetString("site_type") == SiteTypePocketbase {
			runCommand("systemctl", "stop", "pocketbase-"+record.Id)
		}
	} else {
		// Aktif: Pocketbase servisini başlat (gerekliyse) ve Nginx konfigürasyonunu yaz
		if record.GetString("site_type") == SiteTypePocketbase {
			backendPort := 0
			fmt.Sscanf(record.GetString("proxy_url"), "http://127.0.0.1:%d", &backendPort)
			if backendPort > 0 {
				startPocketbaseService(record.Id, backendPort, record.GetString("admin_email"), record.GetString("admin_password"), true)
			} else {
				runCommand("systemctl", "start", "pocketbase-"+record.Id)
			}
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
			SiteID:   record.Id,
		})
		if err != nil {
			return e.JSON(http.StatusOK, map[string]interface{}{
				"record":  record,
				"warning": "nginx config gen failed: " + err.Error(),
			})
		}

		// Clean up old-style domain config if it exists
		ngx.RemoveConfig(domain)

		if err := ngx.WriteConfig(record.Id, config); err != nil {
			return e.JSON(http.StatusOK, map[string]interface{}{
				"record":  record,
				"warning": "nginx config write failed: " + err.Error(),
			})
		}
	}

	ngx.Reload() // best effort

	return e.JSON(http.StatusOK, record)
}

func HandleDeleteSite(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager, pm *PortManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	domain := record.GetString("domain")
	rootDir := record.GetString("root_dir")
	port := record.GetInt("port")

	// Remove Nginx config
	ngx.RemoveConfig(record.Id)
	ngx.RemoveConfig(domain) // Also clean up old-style domain config if it exists

	// Remove web root
	os.RemoveAll(rootDir)

	// Release port in PortManager
	if port > 0 {
		pm.Release(port)
	}

	// If it's a PocketBase site, stop and remove the service
	if record.GetString("site_type") == SiteTypePocketbase {
		stopAndRemovePocketbaseService(record.Id)
		// Extract backend port from proxy_url
		backendPort := 0
		fmt.Sscanf(record.GetString("proxy_url"), "http://127.0.0.1:%d", &backendPort)
		if backendPort > 0 {
			killProcessOnPort(backendPort)
			pm.Release(backendPort)
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
			pm.Release(dbPort)
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
		SiteID:   record.Id,
	})
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Clean up old-style domain config if it exists
	ngx.RemoveConfig(domain)

	if err := ngx.WriteConfig(record.Id, config); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	if record.GetString("site_type") == SiteTypePocketbase {
		backendPort := 0
		fmt.Sscanf(record.GetString("proxy_url"), "http://127.0.0.1:%d", &backendPort)
		if backendPort > 0 {
			startPocketbaseService(record.Id, backendPort, record.GetString("admin_email"), record.GetString("admin_password"), true)
		}
	}

	if err := ngx.Reload(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"message": "site deployed"})
}

// ── Git Deploy (re-deploy from GitHub) ──

func HandleGitDeploy(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	gitRepo := record.GetString("git_repo")
	if gitRepo == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Bu site bir GitHub reposuna bağlı değil."})
	}

	if record.GetString("status") == SiteStatusPaused {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Pasif durumdaki siteler deploy edilemez."})
	}

	// Parse optional overrides from request body and save them
	var req GitDeployRequest
	if err := e.BindBody(&req); err == nil {
		if req.BuildCmd != "" {
			record.Set("build_cmd", req.BuildCmd)
		}
		if req.OutputDir != "" {
			record.Set("output_dir", req.OutputDir)
		}
		_ = app.Save(record)
	}

	// Run clone & build in background
	go func() {
		if err := CloneAndBuild(app, record.Id); err != nil {
			log.Printf("[GitDeploy] Hata (site: %s): %v", record.Id, err)
		} else {
			ngx.Reload()
			log.Printf("[GitDeploy] Başarılı (site: %s)", record.Id)
		}
	}()

	return e.JSON(http.StatusOK, map[string]string{"message": "GitHub deploy başlatıldı. Arka planda çalışıyor."})
}

func HandleGithubWebhook(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	gitRepo := record.GetString("git_repo")
	if gitRepo == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Bu site bir GitHub reposuna bağlı değil."})
	}

	if record.GetString("status") == SiteStatusPaused {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Pasif durumdaki siteler deploy edilemez."})
	}

	// Verify webhook secret (HMAC-SHA256 signature)
	secret := record.GetString("webhook_secret")
	if secret != "" {
		sigHeader := e.Request.Header.Get("X-Hub-Signature-256")
		if sigHeader == "" {
			return e.JSON(http.StatusUnauthorized, map[string]string{"error": "missing signature"})
		}

		body, err := io.ReadAll(e.Request.Body)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read body"})
		}
		// Re-set body so BindBody can read it again
		e.Request.Body = io.NopCloser(bytes.NewReader(body))

		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(expectedSig), []byte(sigHeader)) {
			return e.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
		}
	}

	// Read GitHub event type
	event := e.Request.Header.Get("X-GitHub-Event")
	if event != "" && event != "push" {
		// Return 200 for other events like ping to allow webhook confirmation
		return e.JSON(http.StatusOK, map[string]string{"message": "Event ignored"})
	}

	// Read push branch ref from body
	type GitHubPayload struct {
		Ref string `json:"ref"` // refs/heads/branch_name
	}

	var payload GitHubPayload
	if err := e.BindBody(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	branch := record.GetString("git_branch")
	if branch == "" {
		branch = "main" // default to main
	}

	expectedRef := "refs/heads/" + branch
	if payload.Ref != expectedRef {
		return e.JSON(http.StatusOK, map[string]string{"message": fmt.Sprintf("Push on branch %s ignored. Configured branch is %s", payload.Ref, branch)})
	}

	// Trigger build in background
	go func() {
		if err := CloneAndBuild(app, record.Id); err != nil {
			log.Printf("[Webhook GitDeploy] Hata (site: %s): %v", record.Id, err)
		} else {
			ngx.Reload()
			log.Printf("[Webhook GitDeploy] Başarılı (site: %s)", record.Id)
		}
	}()

	return e.JSON(http.StatusOK, map[string]string{"message": "GitHub Webhook deploy tetiklendi."})
}

// ── SSL ──

func HandleEnableSSL(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager, ssl *SSLManager) error {
	record, err := app.FindRecordById("sites", e.Request.PathValue("id"))
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	domain := record.GetString("domain")

	// Ensure Nginx log directory exists
	os.MkdirAll("/var/log/nginx", 0755)
	sslLogPath := filepath.Join("/var/log/nginx", fmt.Sprintf("%s-ssl.log", domain))

	// Write initial status to log
	initialMsg := fmt.Sprintf("[%s] Let's Encrypt SSL sertifikası talep ediliyor...\n", time.Now().Format("2006-01-02 15:04:05"))
	os.WriteFile(sslLogPath, []byte(initialMsg), 0644)

	// Update status to pending
	record.Set("ssl_status", SSLStatusPending)
	app.Save(record)

	// Issue certificate asynchronously
	go func() {
		result, err := ssl.IssueCertificate(domain, record.GetInt("port"))
		if err != nil {
			// Re-fetch to avoid stale record
			if rec, fetchErr := app.FindRecordById("sites", record.Id); fetchErr == nil {
				rec.Set("ssl_status", SSLStatusError)
				app.Save(rec)
			}

			// Hata detayını log dosyasına yaz
			errMsg := fmt.Sprintf("[%s] SSL Kurulumu Başarısız Oldu:\n%v\n", time.Now().Format("2006-01-02 15:04:05"), err)
			os.WriteFile(sslLogPath, []byte(errMsg), 0644)
			return
		}

		// Re-fetch to avoid overwriting concurrent changes
		rec, fetchErr := app.FindRecordById("sites", record.Id)
		if fetchErr != nil {
			return
		}

		rec.Set("ssl_status", SSLStatusActive)
		rec.Set("ssl_expiry", result.Expiry)
		app.Save(rec)

		// Başarı mesajını log dosyasına yaz
		successMsg := fmt.Sprintf("[%s] SSL Sertifikası Başarıyla Kuruldu!\nSertifika Yolu: %s\nGeçerlilik: %s\n", time.Now().Format("2006-01-02 15:04:05"), result.CertPath, result.Expiry)
		os.WriteFile(sslLogPath, []byte(successMsg), 0644)

		// Regenerate Nginx config with SSL
		config, err := ngx.GenerateConfig(NginxConfigInput{
			Domain:   domain,
			Port:     rec.GetInt("port"),
			RootDir:  rec.GetString("root_dir"),
			SiteType: rec.GetString("site_type"),
			ProxyURL: rec.GetString("proxy_url"),
			SSLEntry: &SSLEntry{CertPath: result.CertPath, KeyPath: result.KeyPath},
			SiteID:   rec.Id,
		})
		if err != nil {
			return
		}

		ngx.RemoveConfig(domain)
		ngx.WriteConfig(rec.Id, config)
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
		SiteID:   record.Id,
	})
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	ngx.RemoveConfig(domain)
	ngx.WriteConfig(record.Id, config)
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
	port, err := pm.Next(app)
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
	go launchPocketbaseInstance(record.Id, port, req.AdminEmail, adminPass, false)

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

func HandleDeleteDatabase(e *core.RequestEvent, app *pocketbase.PocketBase, pm *PortManager) error {
	dbID := e.Request.PathValue("dbId")

	record, err := app.FindRecordById("databases", dbID)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "database not found"})
	}

	// Kill the process on that port
	port := record.GetInt("port")
	if port > 0 {
		killProcessOnPort(port)
		pm.Release(port)
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

func HandleNextPort(e *core.RequestEvent, app *pocketbase.PocketBase, pm *PortManager) error {
	port, err := pm.Next(app)
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

func launchPocketbaseInstance(id string, port int, adminEmail, adminPassword string, localOnly bool) {
	startPocketbaseService(id, port, adminEmail, adminPassword, localOnly)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func startPocketbaseService(id string, port int, adminEmail, adminPassword string, localOnly bool) error {
	host := "0.0.0.0"
	if localOnly {
		host = "127.0.0.1"
	}

	if runtime.GOOS == "windows" {
		cwd, _ := os.Getwd()
		dataDir := filepath.Join(cwd, "databases", id)
		os.MkdirAll(dataDir, 0755)

		executable := "pocketbase"
		if _, err := exec.LookPath("pocketbase"); err != nil {
			localPB := filepath.Join(cwd, "pocketbase.exe")
			if _, errLocal := os.Stat(localPB); errLocal == nil {
				executable = localPB
			} else {
				var errExe error
				executable, errExe = os.Executable()
				if errExe != nil {
					executable = "pocketbase"
				}
			}
		}

		args := []string{"serve", "--dir=" + dataDir, fmt.Sprintf("--http=%s:%d", host, port)}
		migrationsDir := filepath.Join(dataDir, "pb_migrations")
		if info, err := os.Stat(migrationsDir); err == nil && info.IsDir() {
			args = append(args, "--migrationsDir="+migrationsDir)
			log.Printf("[PocketBase] Migrations dizini bulundu: %s", migrationsDir)
		}

		cmd := exec.Command(executable, args...)
		err := cmd.Start()
		if err != nil {
			log.Printf("Failed to start pocketbase locally: %v", err)
			return err
		}

		go func() {
			time.Sleep(2 * time.Second)
			exec.Command(executable, "superuser", "upsert", adminEmail, adminPassword, "--dir="+dataDir).Run()
		}()
		return nil
	}

	dataDir := fmt.Sprintf("/var/lib/dashboard/databases/%s", id)
	os.MkdirAll(dataDir, 0755)

	executable := "/root/nazploy/pocketbase_bin"
	if _, err := os.Stat(executable); os.IsNotExist(err) {
		if _, errPath := exec.LookPath("pocketbase"); errPath == nil {
			executable = "pocketbase"
		} else {
			var errExe error
			executable, errExe = os.Executable()
			if errExe != nil {
				executable = "pocketbase"
			}
		}
	}

	serviceName := fmt.Sprintf("pocketbase-%s", id)

	execStartCmd := fmt.Sprintf("%s serve --dir=%s --http=%s:%d", executable, dataDir, host, port)
	migrationsDir := filepath.Join(dataDir, "pb_migrations")
	if info, err := os.Stat(migrationsDir); err == nil && info.IsDir() {
		execStartCmd += " --migrationsDir=" + migrationsDir
		log.Printf("[PocketBase] Systemd servisine migrationsDir eklendi: %s", migrationsDir)
	}

	serviceFileContent := fmt.Sprintf(`[Unit]
Description=PocketBase Service for %s
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=%s
ExecStart=%s
Restart=always

[Install]
WantedBy=multi-user.target
`, id, dataDir, execStartCmd)

	servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
	if err := os.WriteFile(servicePath, []byte(serviceFileContent), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	runCommand("systemctl", "daemon-reload")
	runCommand("systemctl", "enable", serviceName)
	runCommand("systemctl", "start", serviceName)

	go func() {
		time.Sleep(3 * time.Second)
		exec.Command(executable, "superuser", "upsert", adminEmail, adminPassword, "--dir="+dataDir).Run()
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

func restartPocketbaseService(id string) {
	if runtime.GOOS == "windows" {
		return
	}
	serviceName := fmt.Sprintf("pocketbase-%s", id)
	dataDir := fmt.Sprintf("/var/lib/dashboard/databases/%s", id)

	// Regenerate service file with --migrationsDir if migrations exist
	migrationsDir := filepath.Join(dataDir, "pb_migrations")
	if info, err := os.Stat(migrationsDir); err == nil && info.IsDir() {
		servicePath := fmt.Sprintf("/etc/systemd/system/%s.service", serviceName)
		if existing, err := os.ReadFile(servicePath); err == nil {
			content := string(existing)
			// Only update if --migrationsDir is not already in the service file
			if !strings.Contains(content, "--migrationsDir=") {
				content = strings.Replace(content,
					"Restart=always",
					fmt.Sprintf("# migrationsDir added automatically\nEnvironment=PB_MIGRATIONS_DIR=%s\nRestart=always", migrationsDir),
					1)
				// Find and update ExecStart line to include --migrationsDir
				lines := strings.Split(content, "\n")
				for i, line := range lines {
					if strings.HasPrefix(strings.TrimSpace(line), "ExecStart=") && !strings.Contains(line, "--migrationsDir=") {
						lines[i] = strings.TrimRight(line, "\r\n") + " --migrationsDir=" + migrationsDir
					}
				}
				content = strings.Join(lines, "\n")
				os.WriteFile(servicePath, []byte(content), 0644)
				runCommand("systemctl", "daemon-reload")
				log.Printf("[PocketBase] Servis dosyası güncellendi (migrationsDir eklendi): %s", serviceName)
			}
		}
	}

	log.Printf("[PocketBase] Servis yeniden başlatılıyor (migrations için): %s", serviceName)
	runCommand("systemctl", "restart", serviceName)
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
