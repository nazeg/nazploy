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

		sites.Fields.Add(
			&core.TextField{Name: "git_repo"},
		)

		return app.Save(sites)
	}, func(app core.App) error {
		sites, err := app.FindCollectionByNameOrId("sites")
		if err != nil {
			return err
		}

		sites.Fields.RemoveByName("git_repo")

		return app.Save(sites)
	})
}
