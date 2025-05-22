package database

import (
	"errors"
	"log"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// RunMigrations executes all pending migrations
func RunMigrations(databaseURL string) error {
	log.Println("🔄 Running database migrations...")

	// Create migrate instance
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("✅ No new migrations to run")
			return nil
		}
		return err
	}

	log.Println("✅ Migrations completed successfully")
	return nil
}

// GetMigrationStatus returns current migration version
func GetMigrationStatus(databaseURL string) (uint, bool, error) {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return 0, false, err
	}
	defer m.Close()

	version, dirty, err := m.Version()
	return version, dirty, err
}

// RollbackMigrations rolls back N migrations
func RollbackMigrations(databaseURL string, steps int) error {
	log.Printf("🔄 Rolling back %d migrations...", steps)

	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Steps(-steps); err != nil {
		return err
	}

	log.Println("✅ Rollback completed successfully")
	return nil
}
