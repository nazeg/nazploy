package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	"github.com/vps-dashboard/dashboard/internal/dashboard"
	_ "github.com/vps-dashboard/dashboard/migrations"
)

//go:embed web/dist
var frontendFS embed.FS

func main() {
	app := pocketbase.New()

	// ── Auto-migrate collections ──
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: true,
	})

	// ── Managers ──
	ngx := dashboard.NewNginxManager()
	ssl := dashboard.NewSSLManager()
	pm := dashboard.NewPortManager()

	// ── Custom API routes ──
	app.OnServe().BindFunc(func(se *core.ServeEvent) error {
		// Protected superuser endpoints
		g := se.Router.Group("/api/dashboard")
		g.Bind(apis.RequireSuperuserAuth())

		// Sites
		g.GET("/sites", func(e *core.RequestEvent) error {
			return dashboard.HandleListSites(e, app)
		})
		g.POST("/sites", func(e *core.RequestEvent) error {
			return dashboard.HandleCreateSite(e, app, ngx, pm)
		})
		g.GET("/sites/{id}", func(e *core.RequestEvent) error {
			return dashboard.HandleGetSite(e, app)
		})
		g.PATCH("/sites/{id}", func(e *core.RequestEvent) error {
			return dashboard.HandleUpdateSite(e, app, ngx)
		})
		g.DELETE("/sites/{id}", func(e *core.RequestEvent) error {
			return dashboard.HandleDeleteSite(e, app, ngx)
		})

		// Deploy / reload
		g.POST("/sites/{id}/deploy", func(e *core.RequestEvent) error {
			return dashboard.HandleDeploySite(e, app, ngx)
		})

		// Git deploy (clone & build from GitHub)
		g.POST("/sites/{id}/git-deploy", func(e *core.RequestEvent) error {
			return dashboard.HandleGitDeploy(e, app, ngx)
		})

		// SSL
		g.POST("/sites/{id}/ssl/enable", func(e *core.RequestEvent) error {
			return dashboard.HandleEnableSSL(e, app, ngx, ssl)
		})
		g.POST("/sites/{id}/ssl/disable", func(e *core.RequestEvent) error {
			return dashboard.HandleDisableSSL(e, app, ngx)
		})

		// SSL status check
		g.GET("/sites/{id}/ssl/status", func(e *core.RequestEvent) error {
			return dashboard.HandleSSLStatus(e, app, ssl)
		})

		// Site logs
		g.GET("/sites/{id}/logs", func(e *core.RequestEvent) error {
			return dashboard.HandleGetSiteLogs(e, app)
		})
		g.POST("/sites/{id}/logs/clear", func(e *core.RequestEvent) error {
			return dashboard.HandleClearSiteLogs(e, app)
		})

		// Databases (per-site Pocketbase instances)
		g.POST("/sites/{id}/databases", func(e *core.RequestEvent) error {
			return dashboard.HandleCreateDatabase(e, app, pm)
		})
		g.GET("/sites/{id}/databases", func(e *core.RequestEvent) error {
			return dashboard.HandleListDatabases(e, app)
		})
		g.DELETE("/sites/{id}/databases/{dbId}", func(e *core.RequestEvent) error {
			return dashboard.HandleDeleteDatabase(e, app)
		})

		// Port management
		g.GET("/ports/next", func(e *core.RequestEvent) error {
			return dashboard.HandleNextPort(e, pm)
		})

		// Nginx status
		g.GET("/nginx/status", func(e *core.RequestEvent) error {
			return dashboard.HandleNginxStatus(e, ngx)
		})
		g.POST("/nginx/reload", func(e *core.RequestEvent) error {
			return dashboard.HandleNginxReload(e, ngx)
		})

		// Dashboard stats
		g.GET("/stats", func(e *core.RequestEvent) error {
			return dashboard.HandleStats(e, app)
		})

		// Public Webhook (unauthenticated)
		se.Router.POST("/api/public/webhooks/github/{id}", func(e *core.RequestEvent) error {
			return dashboard.HandleGithubWebhook(e, app, ngx)
		})

		// ── Frontend SPA ──
		distFS, err := fs.Sub(frontendFS, "web/dist")
		if err != nil {
			log.Fatal("Failed to load embedded frontend:", err)
		}

		se.Router.GET("/{path...}", apis.Static(distFS, true))

		return se.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
