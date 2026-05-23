package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		sites, err := app.FindCollectionByNameOrId("sites")
		if err != nil {
			return err
		}

		sites.Fields.Add(&core.TextField{Name: "webhook_secret"})

		return app.Save(sites)
	}, func(app core.App) error {
		sites, err := app.FindCollectionByNameOrId("sites")
		if err != nil {
			return err
		}

		sites.Fields.RemoveByName("webhook_secret")

		return app.Save(sites)
	})
}