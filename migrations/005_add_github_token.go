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

		superusers.Fields.Add(&core.TextField{Name: "github_token"})

		return app.Save(superusers)
	}, func(app core.App) error {
		superusers, err := app.FindCollectionByNameOrId("_superusers")
		if err != nil {
			return err
		}

		superusers.Fields.RemoveByName("github_token")

		return app.Save(superusers)
	})
}
