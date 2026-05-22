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
			&core.TextField{Name: "git_branch"},
			&core.TextField{Name: "build_cmd"},
			&core.TextField{Name: "output_dir"},
			&core.TextField{Name: "git_status"},
			&core.TextField{Name: "git_log"},
		)

		return app.Save(sites)
	}, func(app core.App) error {
		sites, err := app.FindCollectionByNameOrId("sites")
		if err != nil {
			return err
		}

		sites.Fields.RemoveByName("git_branch")
		sites.Fields.RemoveByName("build_cmd")
		sites.Fields.RemoveByName("output_dir")
		sites.Fields.RemoveByName("git_status")
		sites.Fields.RemoveByName("git_log")

		return app.Save(sites)
	})
}
