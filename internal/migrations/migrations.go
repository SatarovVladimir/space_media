package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"sort"
)

//go:embed *.sql
var files embed.FS

func Apply(ctx context.Context, db *sql.DB) error {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return fmt.Errorf("list SQL migrations: %w", err)
	}
	sort.Strings(names)

	for _, name := range names {
		query, err := files.ReadFile(name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := db.ExecContext(ctx, string(query)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
	}

	return nil
}
