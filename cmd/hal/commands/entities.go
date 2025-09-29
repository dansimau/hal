package commands

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/hokaccha/go-prettyjson"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

type EntitySummary struct {
	ID         string
	LastUpdate time.Time
	LogCount   int64
}

// NewEntitiesCmd creates the entities command
func NewEntitiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "entities",
		Aliases: []string{"entity", "ent"},
		Short:   "Display all known entities",
		Long: `Display a table of all known entities in the HAL system.
Shows entity ID, type, last updated timestamp, and count of log entries for each entity.`,
		Example: `  hal entities              # Show all entities
  hal entities show light.kitchen  # Show specific entity state`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runEntitiesCommand()
		},
	}

	// Add show subcommand
	showCmd := &cobra.Command{
		Use:   "show <entity-id>",
		Short: "Show detailed state of a specific entity",
		Long:  `Display the current state of a specific entity in prettified JSON format.`,
		Example: `  hal entities show light.kitchen
  hal entities show sensor.temperature`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShowEntityCommand(args[0])
		},
	}

	cmd.AddCommand(showCmd)
	return cmd
}

func runEntitiesCommand() error {
	// Open database connection using default path
	db, err := store.Open("sqlite.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Get all entities with log counts
	var entities []EntitySummary

	// Query entities and join with log counts
	err = db.Table("entities").
		Select("entities.id, entities.updated_at as last_update, COALESCE(log_counts.count, 0) as log_count").
		Joins("LEFT JOIN (SELECT entity_id, COUNT(*) as count FROM logs GROUP BY entity_id) as log_counts ON entities.id = log_counts.entity_id").
		Order("entities.id").
		Scan(&entities).Error
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	// Print results in table format
	return printEntitiesTable(entities)
}

func printEntitiesTable(entities []EntitySummary) error {
	if len(entities) == 0 {
		fmt.Println("No entities found")
		return nil
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.Header("Entity ID", "Last Updated", "Log Entries")

	for _, entity := range entities {
		err := table.Append(
			entity.ID,
			entity.LastUpdate.Format("2006-01-02 15:04:05"),
			strconv.FormatInt(entity.LogCount, 10),
		)
		if err != nil {
			return fmt.Errorf("failed to append row: %w", err)
		}
	}

	return table.Render()
}

func runShowEntityCommand(entityID string) error {
	// Open database connection using default path
	db, err := store.Open("sqlite.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Query for the specific entity
	var entity store.Entity
	err = db.Where("id = ?", entityID).First(&entity).Error
	if err != nil {
		return fmt.Errorf("entity not found: %s", entityID)
	}

	// Pretty print the entity state as JSON
	return printEntityStateJSON(entity)
}

func printEntityStateJSON(entity store.Entity) error {
	// Use prettyjson to format and colorize the output
	formatter := prettyjson.NewFormatter()
	formatter.Indent = 2

	coloredJSON, err := formatter.Marshal(entity.State)
	if err != nil {
		return fmt.Errorf("failed to format entity data: %w", err)
	}

	fmt.Println(string(coloredJSON))

	return nil
}
