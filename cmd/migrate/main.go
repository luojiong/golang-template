package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"go-server/internal/config"
	"go-server/internal/database"
	"go-server/internal/logger"

	"github.com/google/uuid"
)

func main() {
	var (
		action   = flag.String("action", "", "Migration action (up, down, create, status)")
		name     = flag.String("name", "", "Migration name (for create action)")
		batchID  = flag.String("batch", "", "Batch ID to rollback (for down action)")
		help     = flag.Bool("help", false, "Show help")
	)
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	if *action == "" {
		fmt.Println("Error: action is required")
		showHelp()
		os.Exit(1)
	}

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	loggerManager, err := logger.NewManager(cfg.Logging)
	if err != nil {
		log.Fatalf("Failed to create logger: %v", err)
	}
	if err := loggerManager.Start(); err != nil {
		log.Fatalf("Failed to start logger: %v", err)
	}
	defer loggerManager.Stop()

	loggerInstance := loggerManager.GetLogger("migrate")
	ctx := context.Background()

	// Initialize database
	db, err := database.NewDatabase(cfg, loggerManager)
	if err != nil {
		loggerInstance.Fatal(ctx, "Failed to connect to database", logger.Error(err))
	}
	defer db.Close()

	// Initialize migrator
	migrator := database.NewMigrator(db.DB, loggerInstance, &cfg.Database)

	// Execute action
	switch strings.ToLower(*action) {
	case "up":
		if err := migrator.RunMigrations(); err != nil {
			loggerInstance.Fatal(ctx, "Failed to run migrations", logger.Error(err))
		}
		loggerInstance.Info(ctx, "Migrations completed successfully")

	case "down":
		if *batchID == "" {
			// Get latest batch ID if not specified
			latestBatchID, err := migrator.GetLatestBatchID()
			if err != nil {
				loggerInstance.Fatal(ctx, "Failed to get latest batch ID", logger.Error(err))
			}
			if latestBatchID == "" {
				loggerInstance.Info(ctx, "No migrations to rollback")
				return
			}
			*batchID = latestBatchID
		}

		// Validate batch ID
		if _, err := uuid.Parse(*batchID); err != nil {
			loggerInstance.Fatal(ctx, "Invalid batch ID format", logger.String("batch_id", *batchID))
		}

		if err := migrator.Rollback(*batchID); err != nil {
			loggerInstance.Fatal(ctx, "Failed to rollback migrations", logger.Error(err))
		}
		loggerInstance.Info(ctx, "Rollback completed successfully", logger.String("batch_id", *batchID))

	case "create":
		if *name == "" {
			fmt.Println("Error: name is required for create action")
			showHelp()
			os.Exit(1)
		}
		if err := migrator.CreateMigration(*name, ""); err != nil {
			loggerInstance.Fatal(ctx, "Failed to create migration", logger.Error(err))
		}
		loggerInstance.Info(ctx, "Migration created successfully", logger.String("name", *name))

	case "status":
		migrations, err := migrator.GetMigrationStatus()
		if err != nil {
			loggerInstance.Fatal(ctx, "Failed to get migration status", logger.Error(err))
		}

		if len(migrations) == 0 {
			fmt.Println("No migrations have been applied")
			return
		}

		fmt.Printf("Migration Status (%d migrations applied):\n", len(migrations))
		fmt.Println("==========================================")
		for _, migration := range migrations {
			fmt.Printf("Version: %s\n", migration.Version)
			fmt.Printf("Description: %s\n", migration.Description)
			fmt.Printf("Batch ID: %s\n", migration.BatchID)
			fmt.Printf("Applied At: %s\n", migration.AppliedAt.Format("2006-01-02 15:04:05"))
			fmt.Println("------------------------------------------")
		}

	default:
		fmt.Printf("Error: unknown action '%s'\n", *action)
		showHelp()
		os.Exit(1)
	}
}

func showHelp() {
	fmt.Println("Database Migration Tool")
	fmt.Println("========================")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  migrate -action=<action> [options]")
	fmt.Println()
	fmt.Println("Actions:")
	fmt.Println("  up     - Run all pending migrations")
	fmt.Println("  down   - Rollback the last batch of migrations")
	fmt.Println("  create - Create a new migration file")
	fmt.Println("  status - Show migration status")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  -name <name>    Migration name (required for create action)")
	fmt.Println("  -batch <id>     Batch ID to rollback (optional for down action)")
	fmt.Println("  -help           Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  migrate -action=up")
	fmt.Println("  migrate -action=create -name=add_user_table")
	fmt.Println("  migrate -action=down")
	fmt.Println("  migrate -action=down -batch=550e8400-e29b-41d4-a716-446655440000")
	fmt.Println("  migrate -action=status")
}