package dashboard

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// and copies the build output into targetDir (the site's web root).
// buildCmdOverride and outputDirOverride let the user override auto-detection.
func CloneAndBuild(repo, buildCmdOverride, outputDirOverride, targetDir string) error {
	// Create temp directory for cloning
	tmpDir, err := os.MkdirTemp("", "nazploy-git-*")
	if err != nil {
		return fmt.Errorf("geçici dizin oluşturulamadı: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneDir := filepath.Join(tmpDir, "repo")

	// Clone (shallow, main branch only)
	log.Printf("[GitDeploy] Klonlanıyor: %s", repo)
	cmd := exec.Command("git", "clone", "--depth", "1", repo, cloneDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone başarısız: %w", err)
	}

	// Detect framework
	framework := DetectFramework(cloneDir)
	log.Printf("[GitDeploy] Framework tespit edildi: %s (build: %s, output: %s)", framework.Name, framework.BuildCmd, framework.OutputDir)

	buildCmd := framework.BuildCmd
	outputDir := framework.OutputDir
	if buildCmdOverride != "" {
		buildCmd = buildCmdOverride
	}
	if outputDirOverride != "" {
		outputDir = outputDirOverride
	}

	// Check if package.json exists (it's a Node.js project)
	pkgPath := filepath.Join(cloneDir, "package.json")
	if _, err := os.Stat(pkgPath); err == nil {
		// Install dependencies
		log.Printf("[GitDeploy] npm install çalıştırılıyor...")
		installCmd := exec.Command("npm", "install", "--prefer-offline", "--no-audit", "--no-fund")
		installCmd.Dir = cloneDir
		installCmd.Stdout = os.Stdout
		installCmd.Stderr = os.Stderr
		if err := installCmd.Run(); err != nil {
			return fmt.Errorf("npm install başarısız: %w", err)
		}

		// Run build
		log.Printf("[GitDeploy] Build çalıştırılıyor: %s", buildCmd)
		parts := strings.Fields(buildCmd)
		buildExec := exec.Command(parts[0], parts[1:]...)
		buildExec.Dir = cloneDir
		buildExec.Stdout = os.Stdout
		buildExec.Stderr = os.Stderr

		// For Next.js static export, set output: 'export' env hint
		if framework.Name == "nextjs" {
			buildExec.Env = append(os.Environ(), "NEXT_OUTPUT=export")
		}

		if err := buildExec.Run(); err != nil {
			return fmt.Errorf("build başarısız (%s): %w", buildCmd, err)
		}
	} else {
		// Not a Node.js project — just copy everything as static files
		log.Printf("[GitDeploy] package.json bulunamadı, statik dosya olarak kopyalanıyor...")
		outputDir = "."
	}

	// Determine source directory
	srcDir := filepath.Join(cloneDir, outputDir)
	if _, err := os.Stat(srcDir); os.IsNotExist(err) {
		// Fallback: if output dir doesn't exist, copy the whole repo
		log.Printf("[GitDeploy] Output dizini '%s' bulunamadı, tüm repo kopyalanıyor...", outputDir)
		srcDir = cloneDir
	}

	// Clear existing content in target dir (but keep the dir itself)
	entries, err := os.ReadDir(targetDir)
	if err == nil {
		for _, entry := range entries {
			os.RemoveAll(filepath.Join(targetDir, entry.Name()))
		}
	}

	// Copy build output to target directory
	log.Printf("[GitDeploy] Build çıktısı kopyalanıyor: %s → %s", srcDir, targetDir)
	if err := copyDir(srcDir, targetDir); err != nil {
		return fmt.Errorf("dosya kopyalama başarısız: %w", err)
	}

	log.Printf("[GitDeploy] Deploy tamamlandı: %s", repo)
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
