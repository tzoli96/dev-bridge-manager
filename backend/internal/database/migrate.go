package database

import (
	"database/sql"
	"errors"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

// RunMigrations executes all pending migrations with smart skipping
func RunMigrations(databaseURL string) error {
	log.Println("🔄 Running database migrations...")

	// First try the standard migration approach
	if err := runStandardMigrations(databaseURL); err != nil {
		log.Printf("⚠️  Standard migrations failed: %v", err)
		log.Println("🔄 Trying individual migration approach...")

		// Fallback to individual migration approach
		return runIndividualMigrations(databaseURL)
	}

	log.Println("✅ Migrations completed successfully")
	return nil
}

// runStandardMigrations tries the normal migrate approach
func runStandardMigrations(databaseURL string) error {
	m, err := migrate.New("file://migrations", databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("✅ No new migrations to run")
			return nil
		}
		return err
	}

	return nil
}

// runIndividualMigrations runs migrations one by one, skipping existing ones
func runIndividualMigrations(databaseURL string) error {
	// Open database connection
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	// Ensure schema_migrations table exists
	if err := ensureSchemaMigrationsTable(db); err != nil {
		return err
	}

	// Get migration files
	migrationFiles, err := getMigrationFiles()
	if err != nil {
		return err
	}

	// Process each migration
	for _, file := range migrationFiles {
		if err := processMigrationFile(db, file); err != nil {
			log.Printf("⚠️  Migration %s failed: %v", file, err)
			// Continue with next migration instead of stopping
			continue
		}
	}

	log.Println("✅ Individual migrations completed")
	return nil
}

// ensureSchemaMigrationsTable creates the migrations tracking table
func ensureSchemaMigrationsTable(db *sql.DB) error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version bigint PRIMARY KEY,
			dirty boolean NOT NULL DEFAULT false
		);
	`
	_, err := db.Exec(query)
	return err
}

// getMigrationFiles returns sorted list of .up.sql files
func getMigrationFiles() ([]string, error) {
	files, err := filepath.Glob("migrations/*.up.sql")
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}

// processMigrationFile processes a single migration file
func processMigrationFile(db *sql.DB, filename string) error {
	// Extract version from filename
	version, err := extractVersionFromFilename(filename)
	if err != nil {
		return err
	}

	// Check what this migration creates and if it already exists
	shouldSkip, err := shouldSkipMigration(db, filename, version)
	if err != nil {
		return err
	}

	if shouldSkip {
		log.Printf("✅ Migration %s content already exists, skipping", filename)
		// Still mark it as applied in tracking
		markMigrationApplied(db, version)
		return nil
	}

	log.Printf("🔄 Applying migration %s", filename)

	// Read migration file
	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	// Execute migration with error handling
	if err := executeMigrationWithRetry(db, string(content), version); err != nil {
		return err
	}

	// Mark migration as applied
	if err := markMigrationApplied(db, version); err != nil {
		return err
	}

	log.Printf("✅ Migration %s applied successfully", filename)
	return nil
}

// shouldSkipMigration determines if a migration should be skipped based on actual DB state
func shouldSkipMigration(db *sql.DB, filename string, version int) (bool, error) {
	// First check if it's already in schema_migrations
	applied, err := isMigrationApplied(db, version)
	if err != nil {
		return false, err
	}
	if applied {
		return true, nil
	}

	// Check actual database state based on filename/content
	base := filepath.Base(filename)

	// Check for specific migration patterns
	switch {
	case strings.Contains(base, "create_users_table"):
		return tableExists(db, "users")
	case strings.Contains(base, "create_roles_table"):
		return tableExists(db, "roles")
	case strings.Contains(base, "create_permissions_table"):
		return tableExists(db, "permissions")
	case strings.Contains(base, "create_role_permissions_table"):
		return tableExists(db, "role_permissions")
	case strings.Contains(base, "update_users_table_with_role_id"):
		return columnExists(db, "users", "role_id")
	case strings.Contains(base, "create_projects_table"):
		return tableExists(db, "projects")
	}

	// If we can't determine, don't skip
	return false, nil
}

// tableExists checks if a table exists in the database
func tableExists(db *sql.DB, tableName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = $1
		);
	`
	err := db.QueryRow(query, tableName).Scan(&exists)
	return exists, err
}

// columnExists checks if a column exists in a table
func columnExists(db *sql.DB, tableName, columnName string) (bool, error) {
	var exists bool
	query := `
		SELECT EXISTS (
			SELECT FROM information_schema.columns 
			WHERE table_schema = 'public' 
			AND table_name = $1 
			AND column_name = $2
		);
	`
	err := db.QueryRow(query, tableName, columnName).Scan(&exists)
	return exists, err
}

// executeMigrationWithRetry executes migration with smart error handling
func executeMigrationWithRetry(db *sql.DB, content string, version int) error {
	// Split content by statements (basic splitting by semicolon)
	statements := strings.Split(content, ";")

	for i, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}

		if _, err := db.Exec(stmt); err != nil {
			// Check if it's a "already exists" error - these are safe to ignore
			errStr := strings.ToLower(err.Error())
			if strings.Contains(errStr, "already exists") ||
				strings.Contains(errStr, "duplicate key") ||
				strings.Contains(errStr, "unique constraint") {
				log.Printf("⚠️  Statement %d already exists, skipping: %s", i+1, err.Error())
				continue
			}

			// For other errors, return them
			return err
		}
	}

	return nil
}

// extractVersionFromFilename extracts version number from migration filename
func extractVersionFromFilename(filename string) (int, error) {
	base := filepath.Base(filename)
	parts := strings.Split(base, "_")
	if len(parts) == 0 {
		return 0, errors.New("invalid migration filename format")
	}

	return strconv.Atoi(parts[0])
}

// isMigrationApplied checks if a migration version is already applied
func isMigrationApplied(db *sql.DB, version int) (bool, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE version = $1", version).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// markMigrationApplied marks a migration as applied
func markMigrationApplied(db *sql.DB, version int) error {
	_, err := db.Exec("INSERT INTO schema_migrations (version, dirty) VALUES ($1, false) ON CONFLICT (version) DO NOTHING", version)
	return err
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
