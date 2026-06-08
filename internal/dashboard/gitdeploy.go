package dashboard

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
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
		outputDir := "dist"
		// Try to read vite.config.js or vite.config.ts to detect custom outDir
		for _, cfgName := range []string{"vite.config.js", "vite.config.ts"} {
			cfgPath := filepath.Join(projectDir, cfgName)
			if cfgData, err := os.ReadFile(cfgPath); err == nil {
				re := regexp.MustCompile(`outDir\s*:\s*['"]([^'"]+)['"]`)
				matches := re.FindSubmatch(cfgData)
				if len(matches) > 1 {
					outputDir = string(matches[1])
					break
				}
			}
		}
		return FrameworkInfo{
			Name:      "vite",
			BuildCmd:  "npm run build",
			OutputDir: outputDir,
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

// safeLogBuffer is a thread-safe bytes.Buffer wrapper that writes to server stdout as well.
type safeLogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *safeLogBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	os.Stdout.Write(p) // Print to console for server stdout logs
	return s.buf.Write(p)
}

func (s *safeLogBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// createSecureCommand returns a command configured to run under a non-root user (deployUser) if possible, using sudo -E.
func createSecureCommand(deployUser string, dir string, name string, args ...string) *exec.Cmd {
	var cmd *exec.Cmd
	// Only wrap with sudo on Linux when a valid non-root deployUser is provided
	if runtime.GOOS == "linux" && deployUser != "" && deployUser != "root" {
		sudoArgs := append([]string{"-E", "-u", deployUser, "--", name}, args...)
		cmd = exec.Command("sudo", sudoArgs...)
	} else {
		cmd = exec.Command(name, args...)
	}
	cmd.Dir = dir
	return cmd
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
	var superuser *core.Record
	superusers, err := app.FindAllRecords("_superusers")
	if err == nil && len(superusers) > 0 {
		superuser = superusers[0]
		githubToken = superuser.GetString("github_token")
	}

	// If GitHub App is configured, try to get installation token first
	if superuser != nil {
		appID := superuser.GetString("github_app_id")
		appPem := superuser.GetString("github_app_pem")
		if appID != "" && appPem != "" {
			owner, _, parseErr := ParseGithubOwnerAndRepo(repo)
			if parseErr == nil {
				instToken, tokenErr := GetInstallationTokenForRepo(appID, appPem, owner)
				if tokenErr == nil {
					githubToken = instToken
				} else {
					log.Printf("[GitDeploy] GitHub App token alma hatası (PAT denenecek): %v", tokenErr)
				}
			} else {
				log.Printf("[GitDeploy] GitHub sahibi ayrıştırılamadı: %v", parseErr)
			}
		}
	}

	// Setup environment variables securely (passing token via GIT_CONFIG_COUNT/KEY/VALUE to prevent leakage)
	gitEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	if githubToken != "" && strings.HasPrefix(repo, "https://github.com/") {
		authString := "x-access-token:" + githubToken
		base64Auth := base64.StdEncoding.EncodeToString([]byte(authString))
		gitEnv = append(gitEnv, "GIT_CONFIG_COUNT=1")
		gitEnv = append(gitEnv, "GIT_CONFIG_KEY_0=http.https://github.com/.extraHeader")
		gitEnv = append(gitEnv, "GIT_CONFIG_VALUE_0=Authorization: Basic "+base64Auth)
	}

	var safeBuf safeLogBuffer
	logWrite := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		log.Printf("[GitDeploy] %s", msg)
		safeBuf.Write([]byte(msg + "\n"))
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
	updateSiteStatus("deploying", safeBuf.String())

	// Channel to stop background log writer
	logDone := make(chan struct{})

	// Start background log writer goroutine (updates DB every 2 seconds throttled)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-logDone:
				return
			case <-ticker.C:
				rec, fetchErr := app.FindRecordById("sites", siteID)
				if fetchErr == nil {
					rec.Set("git_log", safeBuf.String())
					_ = app.Save(rec)
				}
			}
		}
	}()

	// Run main build logic inside a wrapper to handle error and log writing
	runBuild := func() error {
		// Create temp directory for cloning
		tmpDir, err := os.MkdirTemp("", "nazploy-git-*")
		if err != nil {
			return fmt.Errorf("geçici dizin oluşturulamadı: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		deployUser := os.Getenv("DEPLOY_USER")
		if deployUser != "" && deployUser != "root" && runtime.GOOS == "linux" {
			// Change ownership of the temp directory to the deploy user so they can write inside it
			chownCmd := exec.Command("chown", "-R", deployUser, tmpDir)
			if err := chownCmd.Run(); err != nil {
				logWrite("[Warning] Geçici dizin sahipliği değiştirilemedi: %v", err)
			}
		}

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

		cmd := createSecureCommand(deployUser, tmpDir, "git", args...)
		cmd.Env = gitEnv
		cmd.Stdout = &safeBuf
		cmd.Stderr = &safeBuf
		if err := runCommandWithTimeout(cmd, 5*time.Minute); err != nil {
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
			installCmd := createSecureCommand(deployUser, cloneDir, "npm", "install", "--prefer-offline", "--no-audit", "--no-fund")
			installCmd.Stdout = &safeBuf
			installCmd.Stderr = &safeBuf
			if err := runCommandWithTimeout(installCmd, 10*time.Minute); err != nil {
				return fmt.Errorf("npm install başarısız: %w", err)
			}

			// Run build
			logWrite("Build çalıştırılıyor: %s...", buildCmd)
			parts := strings.Fields(buildCmd)
			if len(parts) == 0 {
				return fmt.Errorf("geçersiz build komutu")
			}

			// Command Allowlist Validation
			if !isCommandAllowed(parts) {
				return fmt.Errorf("güvenlik hatası: '%s' komutunu veya parametrelerini çalıştırmaya yetkiniz yok. Sadece izin verilen build araçları kullanılabilir", parts[0])
			}

			buildExec := createSecureCommand(deployUser, cloneDir, parts[0], parts[1:]...)
			buildExec.Stdout = &safeBuf
			buildExec.Stderr = &safeBuf

			// Set/append environment variables
			buildExec.Env = os.Environ()
			// For Next.js static export, set output: 'export' env hint
			if framework.Name == "nextjs" {
				buildExec.Env = append(buildExec.Env, "NEXT_OUTPUT=export")
			}

			if err := runCommandWithTimeout(buildExec, 10*time.Minute); err != nil {
				return fmt.Errorf("build başarısız (%s): %w", buildCmd, err)
			}
		} else {
			// Not a Node.js project — just copy everything as static files
			logWrite("package.json bulunamadı, statik dosya olarak kopyalanıyor...")
			outputDir = "."
		}

		// Determine source directory
		srcDir, err := safeJoin(cloneDir, outputDir)
		if err != nil {
			return fmt.Errorf("geçersiz çıktı dizini: %w", err)
		}
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

		if deployUser != "" && deployUser != "root" && runtime.GOOS == "linux" {
			// Change ownership of the target directory back to the deploy user
			_ = exec.Command("chown", "-R", deployUser, targetDir).Run()
		}

		// ── PocketBase migration support ──
		// Copy pb_migrations from the cloned repo to the PocketBase data directory.
		pbMigSrc := filepath.Join(cloneDir, "pb_migrations")
		if info, err := os.Stat(pbMigSrc); err == nil && info.IsDir() {
			rec, _ := app.FindRecordById("sites", siteID)
			if rec != nil && rec.GetString("site_type") == "pocketbase" {
				dbDir := filepath.Join("/var/lib/dashboard/databases", siteID)
				pbMigDst := filepath.Join(dbDir, "pb_migrations")
				os.RemoveAll(pbMigDst)
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

	buildErr := runBuild()

	// Stop background DB writer goroutine
	close(logDone)

	if buildErr != nil {
		logWrite("Hata: %v", buildErr)
		updateSiteStatus("failed", safeBuf.String())
		return buildErr
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

	updateSiteStatus("ready", safeBuf.String())
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

		// Skip symbolic links to prevent security vulnerabilities (symlink traversal/arbitrary file read)
		if entry.Type()&os.ModeSymlink != 0 {
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

// isCommandAllowed checks if the given command and its arguments are in the list of allowed build utilities and parameters
func isCommandAllowed(parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	cmdName := strings.TrimSpace(parts[0])

	// Reject path separators to prevent running local/arbitrary binaries (e.g. ./npm or /tmp/npm)
	if strings.ContainsAny(cmdName, "/\\") || strings.HasPrefix(cmdName, ".") {
		return false
	}

	// Normalize if it contains windows extension
	base := strings.TrimSuffix(cmdName, ".exe")

	allowed := map[string]bool{
		"npm":    true,
		"yarn":   true,
		"pnpm":   true,
		"bun":    true,
		"npx":    true,
		"node":   true,
		"gatsby": true,
		"next":   true,
		"astro":  true,
		"vite":   true,
		"hugo":   true,
		"jekyll": true,
		"make":   true,
		"go":     true,
		"deno":   true,
	}

	if !allowed[base] {
		return false
	}

	// Additional argument checks for security
	args := parts[1:]
	switch base {
	case "node", "deno":
		for _, arg := range args {
			argLower := strings.ToLower(arg)
			// Block interactive mode and inline evaluation
			if argLower == "-e" || argLower == "--eval" ||
				argLower == "-p" || argLower == "--print" ||
				argLower == "-i" || argLower == "--interactive" {
				return false
			}
		}
	case "npx":
		for _, arg := range args {
			argLower := strings.ToLower(arg)
			// Prevent automatic remote package installations/prompts
			if argLower == "-y" || argLower == "--yes" {
				return false
			}
		}
	}

	return true
}

// runCommandWithTimeout executes a command and terminates its process group if it exceeds the timeout limit.
func runCommandWithTimeout(cmd *exec.Cmd, timeout time.Duration) error {
	setProcAttributes(cmd)

	if err := cmd.Start(); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-timer.C:
		killProcessGroup(cmd)
		return fmt.Errorf("komut zaman aşımına uğradı (limit: %v)", timeout)
	case err := <-done:
		return err
	}
}

// safeJoin joins a base path and a relative path, and ensures the result stays inside base.
func safeJoin(base, userPath string) (string, error) {
	joined := filepath.Clean(filepath.Join(base, userPath))
	rel, err := filepath.Rel(base, joined)
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(rel, "..") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("güvenlik hatası: çıktı dizini ana dizinin dışına çıkamaz")
	}
	return joined, nil
}


