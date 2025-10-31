package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"go-server/internal/config"
	"go-server/internal/logger"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Migration represents a database migration
type Migration struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:uuid_generate_v4()"`
	Version     string    `gorm:"size:255;not null;uniqueIndex"` // Migration version
	Description string    `gorm:"size:500"`                      // Migration description
	Up          string    `gorm:"type:text"`                    // SQL for up migration
	Down        string    `gorm:"type:text"`                    // SQL for down migration
	BatchID     uuid.UUID `gorm:"type:uuid;not null"`           // Batch ID for grouping migrations
	AppliedAt   time.Time `gorm:"not null"`                      // When migration was applied
}

// Migrator handles database migrations
type Migrator struct {
	db     *gorm.DB
	logger logger.Logger
	config *config.DatabaseConfig
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *gorm.DB, logger logger.Logger, config *config.DatabaseConfig) *Migrator {
	return &Migrator{
		db:     db,
		logger: logger,
		config: config,
	}
}

// MigrationsDir represents the directory containing migration files
const MigrationsDir = "migrations"

// InitializeMigrations initializes the migrations system
func (m *Migrator) InitializeMigrations() error {
	ctx := context.Background()

	// Create migrations table if it doesn't exist
	if err := m.db.AutoMigrate(&Migration{}); err != nil {
		m.logger.Error(ctx, "Failed to create migrations table", logger.Error(err))
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	m.logger.Info(ctx, "Migrations system initialized", logger.String("table", "migrations"))
	return nil
}

// CreateMigration creates a new migration file
func (m *Migrator) CreateMigration(name, description string) error {
	ctx := context.Background()

	// Generate version from timestamp and name
	version := fmt.Sprintf("%d_%s", time.Now().Unix(), strings.ToLower(strings.ReplaceAll(name, " ", "_")))

	// Ensure migrations directory exists
	if err := os.MkdirAll(MigrationsDir, 0755); err != nil {
		return fmt.Errorf("failed to create migrations directory: %w", err)
	}

	// Generate migration file content
	upSQL := fmt.Sprintf(`-- Migration: %s
-- Description: %s
-- Version: %s

-- Add your UP migration SQL here
-- Example:
-- CREATE TABLE example (
--     id SERIAL PRIMARY KEY,
--     name VARCHAR(255) NOT NULL,
--     created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
-- );
`, description, description, version)

	downSQL := fmt.Sprintf(`-- Migration: %s
-- Description: %s
-- Version: %s

-- Add your DOWN migration SQL here
-- Example:
-- DROP TABLE IF EXISTS example;
`, description, description, version)

	// Write up migration file
	upFile := filepath.Join(MigrationsDir, fmt.Sprintf("%s_up.sql", version))
	if err := os.WriteFile(upFile, []byte(upSQL), 0644); err != nil {
		return fmt.Errorf("failed to create up migration file: %w", err)
	}

	// Write down migration file
	downFile := filepath.Join(MigrationsDir, fmt.Sprintf("%s_down.sql", version))
	if err := os.WriteFile(downFile, []byte(downSQL), 0644); err != nil {
		return fmt.Errorf("failed to create down migration file: %w", err)
	}

	m.logger.Info(ctx, "Created migration files",
		logger.String("version", version),
		logger.String("description", description))
	return nil
}

// RunMigrations runs all pending migrations
func (m *Migrator) RunMigrations() error {
	ctx := context.Background()

	// Initialize migrations system
	if err := m.InitializeMigrations(); err != nil {
		return err
	}

	// Load migration files
	migrations, err := m.loadMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to load migration files: %w", err)
	}

	if len(migrations) == 0 {
		m.logger.Info(ctx, "No migrations to run")
		return nil
	}

	// Get applied migrations
	appliedMigrations, err := m.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Find pending migrations
	pendingMigrations := m.getPendingMigrations(migrations, appliedMigrations)

	if len(pendingMigrations) == 0 {
		m.logger.Info(ctx, "No pending migrations")
		return nil
	}

	// Generate batch ID for this migration run
	batchID := uuid.New()

	// Run pending migrations in order
	for _, migration := range pendingMigrations {
		if err := m.runMigration(migration, batchID); err != nil {
			return fmt.Errorf("failed to run migration %s: %w", migration.Version, err)
		}
	}

	m.logger.Info(ctx, "Successfully ran migrations",
		logger.Int("count", len(pendingMigrations)),
		logger.String("batch_id", batchID.String()))
	return nil
}

// Rollback rolls back the last batch of migrations
func (m *Migrator) Rollback(batchID string) error {
	ctx := context.Background()

	// Get applied migrations for the specified batch
	var migrations []Migration
	if err := m.db.Where("batch_id = ?", batchID).Order("applied_at DESC").Find(&migrations).Error; err != nil {
		return fmt.Errorf("failed to get migrations for batch: %w", err)
	}

	if len(migrations) == 0 {
		m.logger.Info(ctx, "No migrations found for batch", logger.String("batch_id", batchID))
		return nil
	}

	// Rollback migrations in reverse order
	for i := len(migrations) - 1; i >= 0; i-- {
		migration := migrations[i]
		if err := m.rollbackMigration(&migration); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", migration.Version, err)
		}
	}

	m.logger.Info(ctx, "Successfully rolled back migrations",
		logger.Int("count", len(migrations)),
		logger.String("batch_id", batchID))
	return nil
}

// GetMigrationStatus returns the current migration status
func (m *Migrator) GetMigrationStatus() ([]Migration, error) {
	var migrations []Migration
	if err := m.db.Order("applied_at ASC").Find(&migrations).Error; err != nil {
		return nil, fmt.Errorf("failed to get migration status: %w", err)
	}

	return migrations, nil
}

// loadMigrationFiles loads migration files from the migrations directory
func (m *Migrator) loadMigrationFiles() ([]*MigrationFile, error) {
	ctx := context.Background()

	if _, err := os.Stat(MigrationsDir); os.IsNotExist(err) {
		m.logger.Info(ctx, "Migrations directory not found, creating it", logger.String("directory", MigrationsDir))
		if err := os.MkdirAll(MigrationsDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create migrations directory: %w", err)
		}
		return []*MigrationFile{}, nil
	}

	files, err := os.ReadDir(MigrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %w", err)
	}

	var migrationFiles []*MigrationFile
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.HasSuffix(file.Name(), "_up.sql") {
			version := strings.TrimSuffix(file.Name(), "_up.sql")

			// Read up migration
			upPath := filepath.Join(MigrationsDir, file.Name())
			upContent, err := os.ReadFile(upPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read up migration file %s: %w", file.Name(), err)
			}

			// Read down migration
			downPath := filepath.Join(MigrationsDir, strings.Replace(file.Name(), "_up.sql", "_down.sql", 1))
			downContent, err := os.ReadFile(downPath)
			if err != nil {
				return nil, fmt.Errorf("failed to read down migration file for %s: %w", file.Name(), err)
			}

			migrationFiles = append(migrationFiles, &MigrationFile{
				Version: version,
				Up:     string(upContent),
				Down:   string(downContent),
			})
		}
	}

	// Sort migrations by version
	sort.Slice(migrationFiles, func(i, j int) bool {
		return migrationFiles[i].Version < migrationFiles[j].Version
	})

	return migrationFiles, nil
}

// MigrationFile represents a migration file on disk
type MigrationFile struct {
	Version string
	Up      string
	Down    string
}

// getAppliedMigrations returns all applied migrations from the database
func (m *Migrator) getAppliedMigrations() (map[string]bool, error) {
	var migrations []Migration
	if err := m.db.Find(&migrations).Error; err != nil {
		return nil, err
	}

	applied := make(map[string]bool)
	for _, migration := range migrations {
		applied[migration.Version] = true
	}

	return applied, nil
}

// getPendingMigrations returns migrations that haven't been applied yet
func (m *Migrator) getPendingMigrations(files []*MigrationFile, applied map[string]bool) []*MigrationFile {
	var pending []*MigrationFile
	for _, file := range files {
		if !applied[file.Version] {
			pending = append(pending, file)
		}
	}
	return pending
}

// runMigration runs a single migration
func (m *Migrator) runMigration(file *MigrationFile, batchID uuid.UUID) error {
	ctx := context.Background()

	m.logger.Info(ctx, "Running migration", logger.String("version", file.Version))

	// Start transaction
	tx := m.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Execute up migration
	if err := tx.Exec(file.Up).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute up migration: %w", err)
	}

	// Record migration
	migration := Migration{
		Version:     file.Version,
		Description: extractDescription(file.Up),
		Up:          file.Up,
		Down:        file.Down,
		BatchID:     batchID,
		AppliedAt:   time.Now(),
	}

	if err := tx.Create(&migration).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit migration transaction: %w", err)
	}

	m.logger.Info(ctx, "Migration completed successfully", logger.String("version", file.Version))
	return nil
}

// rollbackMigration rolls back a single migration
func (m *Migrator) rollbackMigration(migration *Migration) error {
	ctx := context.Background()

	m.logger.Info(ctx, "Rolling back migration", logger.String("version", migration.Version))

	// Start transaction
	tx := m.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	// Execute down migration
	if err := tx.Exec(migration.Down).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to execute down migration: %w", err)
	}

	// Remove migration record
	if err := tx.Delete(&Migration{}, "version = ?", migration.Version).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit rollback transaction: %w", err)
	}

	m.logger.Info(ctx, "Migration rolled back successfully", logger.String("version", migration.Version))
	return nil
}

// extractDescription extracts description from migration SQL comment
func extractDescription(sql string) string {
	lines := strings.Split(sql, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-- Description:") {
			return strings.TrimPrefix(line, "-- Description:")
		}
	}
	return "No description"
}

// GetLatestBatchID returns the latest batch ID
func (m *Migrator) GetLatestBatchID() (string, error) {
	var migration Migration
	if err := m.db.Order("applied_at DESC").First(&migration).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return migration.BatchID.String(), nil
}