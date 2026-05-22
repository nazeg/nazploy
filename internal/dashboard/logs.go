package dashboard

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func HandleGetSiteLogs(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	siteId := e.Request.PathValue("id")
	if siteId == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "site ID is required"})
	}

	// Fetch site record
	site, err := app.FindRecordById("sites", siteId)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	logType := e.Request.URL.Query().Get("type")
	if logType == "" {
		logType = "nginx_access"
	}

	if logType != "nginx_access" && logType != "nginx_error" && logType != "service" && logType != "ssl" && logType != "git_build" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "invalid log type"})
	}

	if logType == "git_build" {
		logs := site.GetString("git_log")
		if logs == "" {
			logs = "Henüz herhangi bir git build log kaydı bulunmuyor."
		}
		return e.JSON(http.StatusOK, map[string]string{"logs": logs})
	}

	if runtime.GOOS == "windows" {
		mockLogs := fmt.Sprintf("[MOCK LOG - WINDOWS DEV]\n[INFO] Checking logs for site: %s (%s)\n[INFO] Request type: %s\n[SUCCESS] Server is listening locally.", site.GetString("name"), site.GetString("domain"), logType)
		return e.JSON(http.StatusOK, map[string]string{"logs": mockLogs})
	}

	var logs string
	switch logType {
	case "nginx_access":
		filePath := fmt.Sprintf("/var/log/nginx/%s-access.log", site.GetString("domain"))
		logs, err = readLastLines(filePath, 100)
	case "nginx_error":
		filePath := fmt.Sprintf("/var/log/nginx/%s-error.log", site.GetString("domain"))
		logs, err = readLastLines(filePath, 100)
	case "ssl":
		filePath := fmt.Sprintf("/var/log/nginx/%s-ssl.log", site.GetString("domain"))
		logs, err = readLastLines(filePath, 100)
	case "service":
		if site.GetString("site_type") != SiteTypePocketbase {
			return e.JSON(http.StatusBadRequest, map[string]string{"error": "service logs only available for pocketbase sites"})
		}
		serviceName := fmt.Sprintf("pocketbase-%s", site.Id)
		cmd := exec.Command("journalctl", "-u", serviceName, "-n", "100", "--no-pager")
		out, execErr := cmd.Output()
		if execErr != nil {
			logs = fmt.Sprintf("Failed to read systemd logs: %v\nCheck if the systemd service is installed and running.", execErr)
		} else {
			logs = string(out)
		}
	}

	if err != nil {
		logs = fmt.Sprintf("Log dosyası okunamadı: %v", err)
	}

	if logs == "" {
		logs = "Henüz herhangi bir log kaydı bulunmuyor."
	}

	return e.JSON(http.StatusOK, map[string]string{"logs": logs})
}

func readLastLines(filePath string, lineCount int) (string, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "Log dosyası henüz oluşturulmamış (istek gelmemiş olabilir).", nil
	}

	cmd := exec.Command("tail", "-n", fmt.Sprintf("%d", lineCount), filePath)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func HandleClearSiteLogs(e *core.RequestEvent, app *pocketbase.PocketBase) error {
	siteId := e.Request.PathValue("id")
	if siteId == "" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "site ID is required"})
	}

	site, err := app.FindRecordById("sites", siteId)
	if err != nil {
		return e.JSON(http.StatusNotFound, map[string]string{"error": "site not found"})
	}

	logType := e.Request.URL.Query().Get("type")
	if logType != "nginx_access" && logType != "nginx_error" && logType != "ssl" {
		return e.JSON(http.StatusBadRequest, map[string]string{"error": "only nginx_access, nginx_error, and ssl logs can be cleared"})
	}

	if runtime.GOOS == "windows" {
		return e.JSON(http.StatusOK, map[string]string{"message": "mock log cleared"})
	}

	var filePath string
	if logType == "nginx_access" {
		filePath = fmt.Sprintf("/var/log/nginx/%s-access.log", site.GetString("domain"))
	} else if logType == "nginx_error" {
		filePath = fmt.Sprintf("/var/log/nginx/%s-error.log", site.GetString("domain"))
	} else {
		filePath = fmt.Sprintf("/var/log/nginx/%s-ssl.log", site.GetString("domain"))
	}

	if err := os.WriteFile(filePath, []byte(""), 0644); err != nil {
		return e.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to clear log: " + err.Error()})
	}

	return e.JSON(http.StatusOK, map[string]string{"message": "log cleared successfully"})
}
