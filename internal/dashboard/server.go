package dashboard

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

// Secure random key generated at startup for state JWT signing
var githubAppSecret = make([]byte, 32)
var domainRegex = regexp.MustCompile(`^[a-zA-Z0-9][-a-zA-Z0-9.]*[a-zA-Z0-9]$`)

func init() {
	_, _ = rand.Read(githubAppSecret)
}

func isValidDomain(domain string) bool {
	domain = strings.TrimSpace(domain)
	if domain == "" || len(domain) > 253 {
		return false
	}
	// Must match domain regex
	if !domainRegex.MatchString(domain) {
		return false
	}
	// Prevent directory traversal or template injection characters in domain
	if strings.ContainsAny(domain, "\r\n\t'\"`$;<>|{}()[]") {
		return false
	}
	return true
}

func isValidProxyURL(proxyURL string) bool {
	proxyURL = strings.TrimSpace(proxyURL)
	if proxyURL == "" {
		return true
	}
	// Reject newlines or control characters to prevent header injection
	if strings.ContainsAny(proxyURL, "\r\n\t'\"`$;<>|") {
		return false
	}
	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return false
	}
	// Scheme must be http or https
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	// Host must not be empty
	if parsed.Host == "" {
		return false
	}
	return true
}

func generateRandomSecret(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return generateRandomID()
	}
	return hex.EncodeToString(b)
}

func isPortInUse(app *pocketbase.PocketBase, port int, excludeSiteID string) bool {
	records, err := app.FindAllRecords("sites", dbx.NewExp("port = {:port}", dbx.Params{"port": port}))
	if err != nil || len(records) == 0 {
		return false
	}
	if excludeSiteID != "" {
		for _, rec := range records {
			if rec.Id != excludeSiteID {
				return true
			}
		}
		return false
	}
	return true
}

func validateSiteRecord(app *pocketbase.PocketBase, record *core.Record) error {
	name := record.GetString("name")
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("site adı boş olamaz")
	}

	domain := record.GetString("domain")
	if !isValidDomain(domain) {
		return fmt.Errorf("geçersiz domain formatı")
	}

	port := record.GetInt("port")
	if port < PortRangeStart || port > PortRangeEnd {
		return fmt.Errorf("port %d ile %d arasında olmalıdır", PortRangeStart, PortRangeEnd)
	}

	if isPortInUse(app, port, record.Id) {
		return fmt.Errorf("port %d zaten başka bir site tarafından kullanılıyor", port)
	}

	siteType := record.GetString("site_type")
	if siteType != SiteTypeStatic && siteType != SiteTypeProxy && siteType != SiteTypePocketbase {
		return fmt.Errorf("geçersiz site tipi: %s", siteType)
	}

	status := record.GetString("status")
	if status != SiteStatusActive && status != SiteStatusPaused {
		return fmt.Errorf("geçersiz site durumu: %s", status)
	}

	if siteType == SiteTypeProxy {
		pURL := record.GetString("proxy_url")
		if pURL == "" {
			return fmt.Errorf("Proxy sitesi için proxy_url zorunludur")
		}
		if !isValidProxyURL(pURL) {
			return fmt.Errorf("geçersiz proxy url formatı")
		}
	} else if siteType == SiteTypePocketbase {
		pURL := record.GetString("proxy_url")
		if pURL != "" && !isValidProxyURL(pURL) {
			return fmt.Errorf("geçersiz proxy url formatı")
		}
	}

	return nil
}

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

	if req.Name == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}

	domain := strings.TrimSpace(req.Domain)
	if domain == "" {
		host := e.Request.Host
		if idx := strings.Index(host, ":"); idx != -1 {
			host = host[:idx]
		}
		domain = host
	}

	if !isValidDomain(domain) {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "domain formatı geçersiz"})
	}

	if req.SiteType == SiteTypeProxy {
		if req.ProxyURL == "" {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "Proxy sitesi için proxy_url zorunludur"})
		}
		if !isValidProxyURL(req.ProxyURL) {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "proxy_url formatı geçersiz"})
		}
	} else if req.ProxyURL != "" {
		if !isValidProxyURL(req.ProxyURL) {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "proxy_url formatı geçersiz"})
		}
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
	record.Set("domain", domain)
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
		record.Set("webhook_secret", generateRandomSecret(32))
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

	if err := validateSiteRecord(app, record); err != nil {
		if req.Port == 0 {
			pm.Release(port)
		}
		if backendPort > 0 {
			pm.Release(backendPort)
		}
		os.RemoveAll(rootDir)
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
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
		Domain:   domain,
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
	ngx.RemoveConfig(domain)

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
	if req.Domain != nil {
		newDomain := strings.TrimSpace(*req.Domain)
		if newDomain == "" {
			host := e.Request.Host
			if idx := strings.Index(host, ":"); idx != -1 {
				host = host[:idx]
			}
			newDomain = host
		}
		if !isValidDomain(newDomain) {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "domain formatı geçersiz"})
		}
		if newDomain != oldDomain {
			record.Set("domain", newDomain)
			domainChanged = true
		}
	}
	if req.Port != nil {
		record.Set("port", *req.Port)
	}
	if req.SiteType != nil {
		record.Set("site_type", *req.SiteType)
	}
	if req.ProxyURL != nil {
		pURL := strings.TrimSpace(*req.ProxyURL)
		if pURL != "" {
			if !isValidProxyURL(pURL) {
				return e.JSON(http.StatusBadRequest, map[string]string{"error": "proxy_url formatı geçersiz"})
			}
		}
		record.Set("proxy_url", pURL)
	}
	if req.Status != nil {
		record.Set("status", *req.Status)
	}
	if req.Notes != nil {
		record.Set("notes", *req.Notes)
	}
	if req.GitRepo != nil {
		record.Set("git_repo", *req.GitRepo)
		if *req.GitRepo != "" && record.GetString("webhook_secret") == "" {
			record.Set("webhook_secret", generateRandomSecret(32))
		}
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

	// Validate final configuration state before saving
	if err := validateSiteRecord(app, record); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
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
	if secret == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Webhook secret has not been configured. Please save the site settings again to generate one."})
	}

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
	running := false
	if runtime.GOOS == "windows" {
		running = ngx.IsRunning()
	} else {
		cmd := exec.Command("systemctl", "is-active", "nginx")
		err := cmd.Run()
		running = (err == nil)
	}

	statusOutput := ""
	configTest := ""

	if runtime.GOOS == "windows" {
		statusOutput = "Windows üzerinde systemctl kullanılamıyor. Nginx durum testi basit sürümle yapıldı."
		configTest = "Windows üzerinde nginx -t kullanılamıyor."
	} else {
		statusOutput = runCommandOutput("systemctl", "status", "nginx")
		configTest = runCommandOutput("nginx", "-t")
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"running":       running,
		"status_output": statusOutput,
		"config_test":   configTest,
	})
}

func HandleNginxReload(e *core.RequestEvent, ngx *NginxManager) error {
	if err := ngx.Reload(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return e.JSON(http.StatusOK, map[string]string{"message": "nginx reloaded"})
}

func HandleNginxLogs(e *core.RequestEvent) error {
	service := e.Request.URL.Query().Get("service")
	if service == "" {
		service = "nginx"
	}
	if service != "nginx" && service != "nazploy" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid service"})
	}

	linesStr := e.Request.URL.Query().Get("lines")
	lines := 50
	if linesStr != "" {
		if l, err := strconv.Atoi(linesStr); err == nil && l > 0 && l <= 500 {
			lines = l
		}
	}

	logsOutput := ""
	if runtime.GOOS == "windows" {
		logsOutput = fmt.Sprintf("Windows üzerinde journalctl kullanılamıyor. (%s logs)", service)
	} else {
		logsOutput = runCommandOutput("journalctl", "-u", service, "--no-pager", "-n", strconv.Itoa(lines))
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"service": service,
		"logs":    logsOutput,
	})
}

func runCommandOutput(name string, args ...string) string {
	cmd := exec.Command(name, args...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err != nil {
		if out.Len() > 0 {
			return out.String()
		}
		return err.Error()
	}
	return out.String()
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

// ── GitHub App Manifest Integration Endpoints ──

// HandleGithubCallback handles the public redirect from GitHub App manifest creation.
// It exchanges the code for the app's credentials, saves them to the superuser account,
// and redirects the user to install the application.
func HandleGithubCallback(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	code := e.Request.URL.Query().Get("code")
	state := e.Request.URL.Query().Get("state")

	scheme := "http"
	if e.Request.TLS != nil || e.Request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	redirectHost := scheme + "://" + e.Request.Host

	if code == "" {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=missing_code")
	}
	if state == "" {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=missing_state")
	}

	// Verify state token signature
	token, err := jwt.Parse(state, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return githubAppSecret, nil
	})

	if err != nil || !token.Valid {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=invalid_state")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=invalid_state_claims")
	}

	userID, ok := claims["user_id"].(string)
	if !ok || userID == "" {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=invalid_user_in_state")
	}

	// Exchange code for app manifest details
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post("https://api.github.com/app-manifests/"+code+"/conversions", "application/json", nil)
	if err != nil {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=conversion_failed")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=manifest_conversion_failed_status")
	}

	var conversion struct {
		ID            int64  `json:"id"`
		Slug          string `json:"slug"`
		ClientID      string `json:"client_id"`
		ClientSecret  string `json:"client_secret"`
		WebhookSecret string `json:"webhook_secret"`
		PEM           string `json:"pem"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&conversion); err != nil {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=decode_failed")
	}

	// Save to specific superuser
	su, err := app.FindRecordById("_superusers", userID)
	if err != nil {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=superuser_not_found")
	}
	su.Set("github_app_id", fmt.Sprintf("%d", conversion.ID))
	su.Set("github_app_client_id", conversion.ClientID)
	su.Set("github_app_client_secret", conversion.ClientSecret)
	su.Set("github_app_webhook_secret", conversion.WebhookSecret)
	su.Set("github_app_pem", conversion.PEM)
	su.Set("github_app_slug", conversion.Slug)

	if err := app.Save(su); err != nil {
		return e.Redirect(http.StatusTemporaryRedirect, redirectHost+"/settings?github_error=save_failed")
	}

	// Redirect to GitHub App installation flow
	installURL := fmt.Sprintf("https://github.com/apps/%s/installations/new", conversion.Slug)
	return e.Redirect(http.StatusTemporaryRedirect, installURL)
}

// HandleGetGithubRepos fetches the repositories for the configured GitHub App or PAT.
func HandleGetGithubRepos(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	superusers, err := app.FindAllRecords("_superusers")
	if err != nil || len(superusers) == 0 {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Superuser not found"})
	}
	superuser := superusers[0]

	appID := superuser.GetString("github_app_id")
	appPem := superuser.GetString("github_app_pem")
	patToken := superuser.GetString("github_token")

	type RepoInfo struct {
		ID       int64  `json:"id"`
		FullName string `json:"full_name"`
		HTMLURL  string `json:"html_url"`
		Private  bool   `json:"private"`
	}

	var repos []RepoInfo

	// 1. Try App integration
	if appID != "" && appPem != "" {
		jwtToken, err := GenerateAppJWT(appID, appPem)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": "JWT generation failed: " + err.Error()})
		}

		client := &http.Client{Timeout: 15 * time.Second}
		req, _ := http.NewRequest("GET", "https://api.github.com/app/installations", nil)
		req.Header.Set("Authorization", "Bearer "+jwtToken)
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var installations []struct {
					ID int64 `json:"id"`
				}
				json.NewDecoder(resp.Body).Decode(&installations)

				for _, inst := range installations {
					tokenURL := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", inst.ID)
					tReq, _ := http.NewRequest("POST", tokenURL, nil)
					tReq.Header.Set("Authorization", "Bearer "+jwtToken)
					tReq.Header.Set("Accept", "application/vnd.github+json")
					tReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")

					tResp, err := client.Do(tReq)
					if err != nil {
						continue
					}
					var tokenData struct {
						Token string `json:"token"`
					}
					json.NewDecoder(tResp.Body).Decode(&tokenData)
					tResp.Body.Close()

					if tokenData.Token != "" {
						rReq, _ := http.NewRequest("GET", "https://api.github.com/installation/repositories?per_page=100", nil)
						rReq.Header.Set("Authorization", "token "+tokenData.Token)
						rReq.Header.Set("Accept", "application/vnd.github+json")
						rReq.Header.Set("X-GitHub-Api-Version", "2022-11-28")

						rResp, err := client.Do(rReq)
						if err != nil {
							continue
						}
						var repoList struct {
							Repositories []RepoInfo `json:"repositories"`
						}
						json.NewDecoder(rResp.Body).Decode(&repoList)
						rResp.Body.Close()

						repos = append(repos, repoList.Repositories...)
					}
				}
				return e.JSON(http.StatusOK, repos)
			}
		}
	}

	// 2. Fallback to PAT
	if patToken != "" {
		client := &http.Client{Timeout: 15 * time.Second}
		req, _ := http.NewRequest("GET", "https://api.github.com/user/repos?per_page=100&sort=updated", nil)
		req.Header.Set("Authorization", "token "+patToken)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := client.Do(req)
		if err != nil {
			return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			json.NewDecoder(resp.Body).Decode(&repos)
			return e.JSON(http.StatusOK, repos)
		}
		body, _ := io.ReadAll(resp.Body)
		return e.JSON(resp.StatusCode, map[string]string{"error": string(body)})
	}

	return e.JSON(http.StatusOK, repos)
}

// HandleGetGithubBranches fetches the branches for a specific repository.
func HandleGetGithubBranches(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	repoParam := e.Request.URL.Query().Get("repo")
	if repoParam == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "repo parametresi zorunludur"})
	}

	owner, name, err := ParseGithubOwnerAndRepo(repoParam)
	if err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	superusers, err := app.FindAllRecords("_superusers")
	if err != nil || len(superusers) == 0 {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Superuser not found"})
	}
	superuser := superusers[0]

	appID := superuser.GetString("github_app_id")
	appPem := superuser.GetString("github_app_pem")
	patToken := superuser.GetString("github_token")

	token := ""
	if appID != "" && appPem != "" {
		instToken, err := GetInstallationTokenForRepo(appID, appPem, owner)
		if err == nil {
			token = instToken
		}
	}

	if token == "" && patToken != "" {
		token = patToken
	}

	if token == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "GitHub entegrasyonu veya PAT bulunamadı"})
	}

	client := &http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", owner, name)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := client.Do(req)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var branches []interface{}
		json.NewDecoder(resp.Body).Decode(&branches)
		return e.JSON(http.StatusOK, branches)
	}

	body, _ := io.ReadAll(resp.Body)
	return e.JSON(resp.StatusCode, map[string]string{"error": string(body)})
}

// HandleGetGithubAppStatus returns whether a GitHub App is configured.
func HandleGetGithubAppStatus(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	superusers, err := app.FindAllRecords("_superusers")
	if err != nil || len(superusers) == 0 {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Superuser not found"})
	}
	superuser := superusers[0]

	appID := superuser.GetString("github_app_id")
	clientID := superuser.GetString("github_app_client_id")
	slug := superuser.GetString("github_app_slug")

	isConfigured := appID != "" && clientID != ""

	scheme := "http"
	if e.Request.TLS != nil || e.Request.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	webhookURL := fmt.Sprintf("%s://%s/api/public/github/webhook", scheme, e.Request.Host)

	return e.JSON(http.StatusOK, map[string]interface{}{
		"is_configured": isConfigured,
		"app_id":        appID,
		"client_id":     clientID,
		"slug":          slug,
		"webhook_url":   webhookURL,
	})
}

// HandleDisconnectGithubApp clears GitHub App settings.
// HandleGenerateGithubState generates a short-lived state token (JWT) to secure the GitHub App registration callback
func HandleGenerateGithubState(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	var userID string
	if e.Auth != nil {
		userID = e.Auth.Id
	} else {
		superusers, err := app.FindAllRecords("_superusers")
		if err == nil && len(superusers) > 0 {
			userID = superusers[0].Id
		}
	}

	if userID == "" {
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "Yetkisiz erişim"})
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     now.Add(10 * time.Minute).Unix(),
		"iat":     now.Unix(),
	})

	stateToken, err := token.SignedString(githubAppSecret)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "State token üretilemedi"})
	}

	return e.JSON(http.StatusOK, map[string]string{"state": stateToken})
}

func HandleDisconnectGithubApp(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	superusers, err := app.FindAllRecords("_superusers")
	if err != nil || len(superusers) == 0 {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Superuser not found"})
	}
	superuser := superusers[0]

	superuser.Set("github_app_id", "")
	superuser.Set("github_app_client_id", "")
	superuser.Set("github_app_client_secret", "")
	superuser.Set("github_app_webhook_secret", "")
	superuser.Set("github_app_pem", "")
	superuser.Set("github_app_slug", "")

	if err := app.Save(superuser); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Kaydedilemedi: " + err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"message": "GitHub App bağlantısı kesildi."})
}

// HandleGithubAppWebhook handles global push events from GitHub App.
func HandleGithubAppWebhook(e *core.RequestEvent, app *pocketbase.PocketBase, ngx *NginxManager) error {
	superusers, err := app.FindAllRecords("_superusers")
	if err != nil || len(superusers) == 0 {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Superuser not found"})
	}
	superuser := superusers[0]

	webhookSecret := superuser.GetString("github_app_webhook_secret")

	// Verify signature strictly (must be configured)
	if webhookSecret == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "GitHub App webhook secret is not configured"})
	}

	sigHeader := e.Request.Header.Get("X-Hub-Signature-256")
	if sigHeader == "" {
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "missing signature"})
	}

	body, err := io.ReadAll(e.Request.Body)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to read body"})
	}
	// Restore body
	e.Request.Body = io.NopCloser(bytes.NewReader(body))

	mac := hmac.New(sha256.New, []byte(webhookSecret))
	mac.Write(body)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expectedSig), []byte(sigHeader)) {
		return e.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid signature"})
	}

	event := e.Request.Header.Get("X-GitHub-Event")
	if event != "" && event != "push" {
		return e.JSON(http.StatusOK, map[string]string{"message": "Event ignored"})
	}

	// Parse payload
	var payload struct {
		Ref        string `json:"ref"` // refs/heads/branch
		Repository struct {
			HTMLURL string `json:"html_url"`
		} `json:"repository"`
	}

	if err := e.BindBody(&payload); err != nil {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}

	repoURL := strings.TrimSpace(payload.Repository.HTMLURL)
	if repoURL == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "missing repository url"})
	}

	normRepoURL := strings.TrimSuffix(repoURL, ".git")

	// Find matching active sites
	records, err := app.FindAllRecords("sites", dbx.NewExp("status = 'active' AND git_repo != ''"))
	if err != nil {
		return e.JSON(http.StatusOK, map[string]string{"message": "No active sites found"})
	}

	triggeredCount := 0
	for _, record := range records {
		siteRepo := strings.TrimSuffix(strings.TrimSpace(record.GetString("git_repo")), ".git")
		if strings.EqualFold(siteRepo, normRepoURL) {
			branch := record.GetString("git_branch")
			if branch == "" {
				branch = "main"
			}
			expectedRef := "refs/heads/" + branch

			if payload.Ref == expectedRef {
				triggeredCount++
				go func(siteID string) {
					if err := CloneAndBuild(app, siteID); err != nil {
						log.Printf("[App Webhook GitDeploy] Hata (site: %s): %v", siteID, err)
					} else {
						ngx.Reload()
						log.Printf("[App Webhook GitDeploy] Başarılı (site: %s)", siteID)
					}
				}(record.Id)
			}
		}
	}

	return e.JSON(http.StatusOK, map[string]interface{}{
		"message":   "Webhook processed",
		"triggered": triggeredCount,
	})
}

// HandleSystemUpdate triggers a self-update by running the setup.sh script in a transient systemd scope/service.
func HandleSystemUpdate(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	if runtime.GOOS == "windows" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "Windows üzerinde otomatik güncelleme desteklenmiyor."})
	}

	// Try to find the script in common locations (/root/nazploy-src/setup.sh)
	scriptPath := "/root/nazploy-src/setup.sh"
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		// Fallback to current working directory setup.sh if it exists
		cwd, _ := os.Getwd()
		localPath := filepath.Join(cwd, "setup.sh")
		if _, errLocal := os.Stat(localPath); errLocal == nil {
			scriptPath = localPath
		} else {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "setup.sh betiği bulunamadı: " + scriptPath})
		}
	}

	if _, err := exec.LookPath("systemd-run"); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Sistem üzerinde 'systemd-run' komutu bulunamadı. Servis cgroup dışına çıkarılamıyor."})
	}

	// Clean up old log file before running a new update
	os.Remove("/tmp/nazploy_install.log")

	// Trigger systemd-run to run the script in a separate service cgroup.
	// This prevents the update script from being killed when it restarts the nazploy service itself.
	cmd := exec.Command("systemd-run",
		"--description=Nazploy Self Update",
		"bash", scriptPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Güncelleme başlatılamadı: " + err.Error() + " (Stderr: " + stderr.String() + ")",
		})
	}

	return e.JSON(http.StatusOK, map[string]string{
		"message": "Güncelleme arka planda başlatıldı. Sunucu birkaç dakika içinde güncellenip yeniden başlatılacaktır.",
	})
}

// HandleSystemUpdateLogs reads and returns the contents of /tmp/nazploy_install.log.
func HandleSystemUpdateLogs(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	logPath := "/tmp/nazploy_install.log"
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return e.JSON(http.StatusOK, map[string]string{"logs": "Güncelleme günlüğü henüz oluşturulmadı..."})
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "Günlük dosyası okunamadı: " + err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"logs": string(data)})
}


