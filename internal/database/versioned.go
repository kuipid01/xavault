package database

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type schemaMigration struct {
	Version   string    `gorm:"primaryKey;type:varchar(180)"`
	AppliedAt time.Time `gorm:"not null"`
}

func (schemaMigration) TableName() string { return "schema_migrations" }

func RunVersionedMigrations(db *gorm.DB) error {
	if err := db.AutoMigrate(&schemaMigration{}); err != nil {
		return fmt.Errorf("create schema migration table: %w", err)
	}
	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		version := entry.Name()
		if err := db.Transaction(func(tx *gorm.DB) error {
			var existing schemaMigration
			err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&existing, "version = ?", version).Error
			if err == nil {
				return nil
			}
			if err != gorm.ErrRecordNotFound {
				return err
			}
			sqlBytes, err := migrationFiles.ReadFile("migrations/" + version)
			if err != nil {
				return err
			}
			if err := tx.Exec(string(sqlBytes)).Error; err != nil {
				return fmt.Errorf("apply migration %s: %w", version, err)
			}
			return tx.Create(&schemaMigration{Version: version, AppliedAt: time.Now()}).Error
		}); err != nil {
			return err
		}
	}
	return nil
}
