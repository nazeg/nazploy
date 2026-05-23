package dashboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/pocketbase/pocketbase/core"
)

// ── Framework Detection ──

type FrameworkInfo struct {
	Name      string
	BuildCmd  string
	OutputDir string
}

// DetectFramework reads package.json and determines the framework, build command, and output directory.
func DetectFramework(projectDir string) FrameworkInfo {
	pkgPath := filepath.Join(projectDir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return FrameworkInfo{Name: "unknown", BuildCmd: "npm run build", OutputDir: "dist"}
	}

	var pkg struct {
		Scripts         map[string]string `json:"scripts"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return FrameworkInfo{Name: "unknown", BuildCmd: "npm run build", OutputDir: "dist"}
	}

	// Merge deps for easier lookup
	allDeps := make(map[string]bool)
	for k := range pkg.Dependencies {
		allDeps[k] = true
	}
	for k := range pkg.DevDependencies {
		allDeps[k] = true
	}

	// Next.js
	if allDeps["next"] {
		return FrameworkInfo{
			Name:      "nextjs",
			BuildCmd:  "npm run build",
			OutputDir: "out",
		}
	}

	// Astro
	if allDeps["astro"] {
		return FrameworkInfo{
			Name:      "astro",
			BuildCmd:  "npm run build",
			OutputDir: "dist",
		}
	}

	// Vite (React, Vue, Svelte, etc.)
	if allDeps["vite"] || allDeps["@vitejs/plugin-react"] || allDeps["@vitejs/plugin-vue"] || allDeps["@vitejs/plugin-svelte"] {
		return FrameworkInfo{
			Name:      "vite",
			BuildCmd:  "npm run build",
			OutputDir: "dist",
		}
	}

	// Create React App (legacy)
	if allDeps["react-scripts"] {
		return FrameworkInfo{
			Name:      "cra",
			BuildCmd:  "npm run build",
			OutputDir: "build",
		}
	}

	// Generic fallback
	return FrameworkInfo{
		Name:      "generic",
		BuildCmd:  "npm run build",
		OutputDir: "dist",
	}
}

// ── Clone & Build ──

// CloneAndBuild clones a GitHub repo, detects the framework, builds it,
// and copies the build output into the site's web root.
// It logs build details to the site's git_log and updates git_status.
func CloneAndBuild(app core.App, siteID string) error {
	record, err := app.FindRecordById("sites", siteID)
	if err != nil {
		return fmt.Errorf("site kaydı bulunamadı: %w", err)
	}

	repo := record.GetString("git_repo")
	branch := record.GetString("git_branch")
	buildCmdOverride := record.GetString("build_cmd")
	outputDirOverride := record.GetString("output_dir")
	targetDir := record.GetString("root_dir")

	if repo == "" {
		return fmt.Errorf("siteye tanımlı bir git deposu bulunmuyor")
	}

	// Try to get GitHub Token or App Credentials from superusers collection
	githubToken := ""
	var superuser core.Record
	superusers, err := app.FindAllRecords("_superusers")
	if err == nil && len(superusers) > 0 {
		superuser = *superusers[0]
		githubToken = superuser.GetString("github_token")
	}

	// If GitHub App is configured, try to get installation token first
	appID := superuser.GetString("github_app_id")
	appPem := superuser.GetString("github_app_pem")
	isAppToken := false
	if appID != "" && appPem != "" {
		owner, _, parseErr := ParseGithubOwnerAndRepo(repo)
		if parseErr == nil {
			instToken, tokenErr := GetInstallationTokenForRepo(appID, appPem, owner)
			if tokenErr == nil {
				githubToken = instToken
				isAppToken = true
			} else {
				log.Printf("[GitDeploy] GitHub App token alma hatası (PAT denenecek): %v", tokenErr)
			}
		} else {
			log.Printf("[GitDeploy] GitHub sahibi ayrıştırılamadı: %v", parseErr)
		}
	}

	// Rewrite GitHub URL to include token if present
	if githubToken != "" && strings.HasPrefix(repo, "https://github.com/") {
		if isAppToken {
			repo = strings.Replace(repo, "https://github.com/", fmt.Sprintf("https://x-access-token:%s@github.com/", githubToken), 1)
		} else {
			repo = strings.Replace(repo, "https://github.com/", fmt.Sprintf("https://%s@github.com/", githubToken), 1)
		}
	}

	var logBuf bytes.Buffer
	logWrite := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		log.Printf("[GitDeploy] %s", msg)
		logBuf.WriteString(msg + "\n")
	}

	updateSiteStatus := func(status string, logs string) {
		rec, err := app.FindRecordById("sites", siteID)
		if err == nil {
			rec.Set("git_status", status)
			rec.Set("git_log", logs)
			_ = app.Save(rec)
		}
	}

	// Set status to deploying
	logWrite("Deploy işlemi başlatıldı. Site: %s (%s)", record.GetString("name"), record.GetString("domain"))
	updateSiteStatus("deploying", logBuf.String())

	// Run main build logic inside a wrapper to handle error and log writing
	runBuild := func() error {
		// Create temp directory for cloning
		tmpDir, err := os.MkdirTemp("", "nazploy-git-*")
		if err != nil {
			return fmt.Errorf("geçici dizin oluşturulamadı: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		cloneDir := filepath.Join(tmpDir, "repo")

		// Clone (shallow, specific branch if provided)
		logWrite("Klonlanıyor: %s (Branch: %s)...", repo, func() string {
			if branch != "" {
				return branch
			}
			return "varsayılan"
		}())

		args := []string{"clone", "--depth", "1"}
		if branch != "" {
			args = append(args, "--branch", branch)
		}
		args = append(args, repo, cloneDir)

		cmd := exec.Command("git", args...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		cmd.Stdout = io.MultiWriter(os.Stdout, &logBuf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &logBuf)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("git clone başarısız: %w", err)
		}

		// Detect framework
		framework := DetectFramework(cloneDir)
		logWrite("Framework tespit edildi: %s (Önerilen build: %s, output: %s)", framework.Name, framework.BuildCmd, framework.OutputDir)

		buildCmd := framework.BuildCmd
		outputDir := framework.OutputDir
		if buildCmdOverride != "" {
			buildCmd = buildCmdOverride
			logWrite("Kullanıcı build komutu override: %s", buildCmd)
		}
		if outputDirOverride != "" {
			outputDir = outputDirOverride
			logWrite("Kullanıcı output dizini override: %s", outputDir)
		}

		// Check if package.json exists (it's a Node.js project)
		pkgPath := filepath.Join(cloneDir, "package.json")
		if _, err := os.Stat(pkgPath); err == nil {
			// Install dependencies
			logWrite("npm install çalıştırılıyor...")
			installCmd := exec.Command("npm", "install", "--prefer-offline", "--no-audit", "--no-fund")
			installCmd.Dir = cloneDir
			installCmd.Stdout = io.MultiWriter(os.Stdout, &logBuf)
			installCmd.Stderr = io.MultiWriter(os.Stderr, &logBuf)
			if err := installCmd.Run(); err != nil {
				return fmt.Errorf("npm install başarısız: %w", err)
			}

			// Run build
			logWrite("Build çalıştırılıyor: %s...", buildCmd)
			parts := strings.Fields(buildCmd)
			if len(parts) == 0 {
				return fmt.Errorf("geçersiz build komutu")
			}
			buildExec := exec.Command(parts[0], parts[1:]...)
			buildExec.Dir = cloneDir
			buildExec.Stdout = io.MultiWriter(os.Stdout, &logBuf)
			buildExec.Stderr = io.MultiWriter(os.Stderr, &logBuf)

			// For Next.js static export, set output: 'export' env hint
			if framework.Name == "nextjs" {
				buildExec.Env = append(os.Environ(), "NEXT_OUTPUT=export")
			}

			if err := buildExec.Run(); err != nil {
				return fmt.Errorf("build başarısız (%s): %w", buildCmd, err)
			}
		} else {
			// Not a Node.js project — just copy everything as static files
			logWrite("package.json bulunamadı, statik dosya olarak kopyalanıyor...")
			outputDir = "."
		}

		// Determine source directory
		srcDir := filepath.Join(cloneDir, outputDir)
		if _, err := os.Stat(srcDir); os.IsNotExist(err) {
			// Fallback: if output dir doesn't exist, copy the whole repo
			logWrite("Output dizini '%s' bulunamadı, tüm repo kopyalanıyor...", outputDir)
			srcDir = cloneDir
		}

		// Clear existing content in target dir (but keep the dir itself)
		logWrite("Hedef dizin temizleniyor: %s", targetDir)
		entries, err := os.ReadDir(targetDir)
		if err == nil {
			for _, entry := range entries {
				os.RemoveAll(filepath.Join(targetDir, entry.Name()))
			}
		}

		// Copy build output to target directory
		logWrite("Build çıktısı kopyalanıyor: %s → %s", srcDir, targetDir)
		if err := copyDir(srcDir, targetDir); err != nil {
			return fmt.Errorf("dosya kopyalama başarısız: %w", err)
		}

		// ── PocketBase migration support ──
		// Copy pb_migrations from the cloned repo to the PocketBase data directory.
		// This must happen inside runBuild while cloneDir still exists (before defer cleanup).
		pbMigSrc := filepath.Join(cloneDir, "pb_migrations")
		if info, err := os.Stat(pbMigSrc); err == nil && info.IsDir() {
			rec, _ := app.FindRecordById("sites", siteID)
			if rec != nil && rec.GetString("site_type") == "pocketbase" {
				dbDir := filepath.Join("/var/lib/dashboard/databases", siteID)
				pbMigDst := filepath.Join(dbDir, "pb_migrations")
				os.MkdirAll(pbMigDst, 0755)

				if err := copyDir(pbMigSrc, pbMigDst); err != nil {
					logWrite("pb_migrations kopyalama hatası: %v", err)
				} else {
					logWrite("pb_migrations kopyalandı → %s", pbMigDst)
				}
			}
		}

		logWrite("Deploy başarıyla tamamlandı!")
		return nil
	}

	if err := runBuild(); err != nil {
		logWrite("Hata: %v", err)
		updateSiteStatus("failed", logBuf.String())
		return err
	}

	// If migrations were copied, restart PocketBase service so they are applied
	rec, _ := app.FindRecordById("sites", siteID)
	if rec != nil && rec.GetString("site_type") == "pocketbase" {
		dbDir := filepath.Join("/var/lib/dashboard/databases", siteID)
		pbMigDir := filepath.Join(dbDir, "pb_migrations")
		if info, err := os.Stat(pbMigDir); err == nil && info.IsDir() {
			restartPocketbaseService(siteID)
		}
	}

	updateSiteStatus("ready", logBuf.String())
	return nil
}

// ── File Copy Helpers ──

func copyDir(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	os.MkdirAll(dst, 0755)

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		// Skip .git, node_modules
		if entry.Name() == ".git" || entry.Name() == "node_modules" {
			continue
		}

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// ── GitHub App Helpers ──

// ParseGithubOwnerAndRepo parses the owner and repository name from various GitHub URL formats.
func ParseGithubOwnerAndRepo(repoURL string) (string, string, error) {
	// Normalize URL by trimming spaces and suffix
	u := strings.TrimSpace(repoURL)
	u = strings.TrimSuffix(u, ".git")

	// HTTPS: https://github.com/owner/repo
	if strings.Contains(u, "github.com/") {
		parts := strings.Split(u, "github.com/")
		if len(parts) > 1 {
			subparts := strings.Split(parts[1], "/")
			if len(subparts) >= 2 {
				return subparts[0], subparts[1], nil
			}
		}
	}

	// SSH: git@github.com:owner/repo
	if strings.Contains(u, "github.com:") {
		parts := strings.Split(u, "github.com:")
		if len(parts) > 1 {
			subparts := strings.Split(parts[1], "/")
			if len(subparts) >= 2 {
				return subparts[0], subparts[1], nil
			}
		}
	}

	return "", "", fmt.Errorf("GitHub sahibi/reposu URL'den ayrıştırılamadı: %s", repoURL)
}

// GenerateAppJWT creates a signed JWT for GitHub App authentication.
func GenerateAppJWT(appID string, privateKeyPEM string) (string, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM([]byte(privateKeyPEM))
	if err != nil {
		return "", fmt.Errorf("RSA private key ayrıştırılamadı: %w", err)
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iat": now.Add(-60 * time.Second).Unix(), // 60 seconds buffer for clock drift
		"exp": now.Add(9 * time.Minute).Unix(),   // 10 minutes maximum
		"iss": appID,
	})

	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("JWT imzalanırken hata oluştu: %w", err)
	}

	return tokenString, nil
}

// GetInstallationTokenForRepo retrieves an installation token for a given repository owner/org.
func GetInstallationTokenForRepo(appID string, pem string, owner string) (string, error) {
	jwtToken, err := GenerateAppJWT(appID, pem)
	if err != nil {
		return "", err
	}

	// 1. List all installations of this GitHub App
	req, err := http.NewRequest("GET", "https://api.github.com/app/installations", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GitHub API bağlantı hatası: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub installations sorgusu başarısız (%d): %s", resp.StatusCode, string(body))
	}

	var installations []struct {
		ID      int64 `json:"id"`
		Account struct {
			Login string `json:"login"`
		} `json:"account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&installations); err != nil {
		return "", fmt.Errorf("installations response JSON ayrıştırılamadı: %w", err)
	}

	var installationID int64
	for _, inst := range installations {
		if strings.EqualFold(inst.Account.Login, owner) {
			installationID = inst.ID
			break
		}
	}

	if installationID == 0 {
		// Fallback to the first installation if login mismatch occurs but at least one installation exists
		if len(installations) > 0 {
			installationID = installations[0].ID
		} else {
			return "", fmt.Errorf("GitHub organizasyon/kullanıcı '%s' için aktif uygulama yüklemesi bulunamadı", owner)
		}
	}

	// 2. Request installation access token
	tokenURL := fmt.Sprintf("https://api.github.com/app/installations/%d/access_tokens", installationID)
	req, err = http.NewRequest("POST", tokenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+jwtToken)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err = client.Do(req)
	if err != nil {
		return "", fmt.Errorf("GitHub access token talebi başarısız: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("GitHub token üretilemedi (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("token response JSON ayrıştırılamadı: %w", err)
	}

	return tokenResp.Token, nil
}

