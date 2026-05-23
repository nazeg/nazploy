package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		superusers, err := app.FindCollectionByNameOrId("_superusers")
		if err != nil {
			return err
		}

		superusers.Fields.Add(
			&core.TextField{Name: "github_app_id"},
			&core.TextField{Name: "github_app_client_id"},
			&core.TextField{Name: "github_app_client_secret"},
			&core.TextField{Name: "github_app_webhook_secret"},
			&core.TextField{Name: "github_app_pem"},
			&core.TextField{Name: "github_app_slug"},
		)

		return app.Save(superusers)
	}, func(app core.App) error {
		superusers, err := app.FindCollectionByNameOrId("_superusers")
		if err != nil {
			return err
		}

		superusers.Fields.RemoveByName("github_app_id")
		superusers.Fields.RemoveByName("github_app_client_id")
		superusers.Fields.RemoveByName("github_app_client_secret")
		superusers.Fields.RemoveByName("github_app_webhook_secret")
		superusers.Fields.RemoveByName("github_app_pem")
		superusers.Fields.RemoveByName("github_app_slug")

		return app.Save(superusers)
	})
}
