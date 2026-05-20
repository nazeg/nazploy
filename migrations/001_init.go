package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		// ── Sites collection ──
		sitesCollection := core.NewBaseCollection("sites")

		sitesCollection.ListRule = types.Pointer("@request.auth.id != ''")
		sitesCollection.ViewRule = types.Pointer("@request.auth.id != ''")
		sitesCollection.CreateRule = types.Pointer("@request.auth.id != ''")
		sitesCollection.UpdateRule = types.Pointer("@request.auth.id != ''")
		sitesCollection.DeleteRule = types.Pointer("@request.auth.id != ''")

		sitesCollection.Fields.Add(
			&core.TextField{Name: "name", Required: true},
			&core.TextField{Name: "domain", Required: true},
			&core.NumberField{Name: "port", Required: true},
			&core.TextField{Name: "root_dir"},
			&core.SelectField{Name: "site_type", Values: []string{"static", "proxy", "pocketbase"}, MaxSelect: 1},
			&core.TextField{Name: "proxy_url"},
			&core.EmailField{Name: "admin_email"},
			&core.TextField{Name: "admin_password"},
			&core.SelectField{Name: "ssl_status", Values: []string{"none", "pending", "active", "error"}, MaxSelect: 1},
			&core.TextField{Name: "ssl_expiry"},
			&core.SelectField{Name: "status", Values: []string{"active", "paused"}, MaxSelect: 1},
			&core.TextField{Name: "notes"},
		)

		sitesCollection.AddIndex("idx_sites_domain", true, "domain", "")

		if err := app.Save(sitesCollection); err != nil {
			return err
		}

		// ── Databases collection ──
		databasesCollection := core.NewBaseCollection("databases")

		databasesCollection.ListRule = types.Pointer("@request.auth.id != ''")
		databasesCollection.ViewRule = types.Pointer("@request.auth.id != ''")
		databasesCollection.CreateRule = types.Pointer("@request.auth.id != ''")
		databasesCollection.UpdateRule = types.Pointer("@request.auth.id != ''")
		databasesCollection.DeleteRule = types.Pointer("@request.auth.id != ''")

		databasesCollection.Fields.Add(
			&core.RelationField{
				Name:          "site_id",
				Required:      true,
				CollectionId:  sitesCollection.Id,
				CascadeDelete: true,
			},
			&core.TextField{Name: "name", Required: true},
			&core.TextField{Name: "db_type"},
			&core.NumberField{Name: "port", Required: true},
			&core.EmailField{Name: "admin_email"},
			&core.TextField{Name: "admin_password"},
			&core.SelectField{Name: "status", Values: []string{"active", "paused"}, MaxSelect: 1},
		)

		if err := app.Save(databasesCollection); err != nil {
			return err
		}

		return nil
	}, func(app core.App) error {
		// Down migration: remove collections
		if sites, _ := app.FindCollectionByNameOrId("sites"); sites != nil {
			app.Delete(sites)
		}
		if databases, _ := app.FindCollectionByNameOrId("databases"); databases != nil {
			app.Delete(databases)
		}
		return nil
	})
}
