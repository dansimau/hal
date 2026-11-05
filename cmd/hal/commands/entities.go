package commands

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/dansimau/hal/store"
	"github.com/hokaccha/go-prettyjson"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

// errEntityNotFound is returned when the requested entity is not in the database.
var errEntityNotFound = errors.New("entity not found")

type EntitySummary struct {
	ID         string
	LastUpdate time.Time
	LogCount   int64
}

// NewEntitiesCmd creates the entities command.
func NewEntitiesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "entities",
		Aliases: []string{"entity", "ent"},
		Short:   "Display all known entities",
		Long: `Display a table of all known entities in the HAL system.
Shows entity ID, type, last updated timestamp, and count of log entries for each entity.`,
		Example: `  hal entities              # Show all entities
  hal entities show light.kitchen  # Show specific entity state`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runEntitiesCommand(cmd.OutOrStdout())
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
			return runShowEntityCommand(cmd.OutOrStdout(), args[0])
		},
	}

	cmd.AddCommand(showCmd)

	return cmd
}

func runEntitiesCommand(out io.Writer) error {
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
		Joins("LEFT JOIN (SELECT entity_id, COUNT(*) as count FROM logs GROUP BY entity_id) as log_counts " +
			"ON entities.id = log_counts.entity_id").
		Order("entities.id").
		Scan(&entities).Error
	if err != nil {
		return fmt.Errorf("failed to query entities: %w", err)
	}

	// Print results in table format
	return printEntitiesTable(out, entities)
}

func printEntitiesTable(out io.Writer, entities []EntitySummary) error {
	if len(entities) == 0 {
		fmt.Fprintln(out, "No entities found")

		return nil
	}

	table := tablewriter.NewWriter(out)
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

func runShowEntityCommand(out io.Writer, entityID string) error {
	// Open database connection using default path
	db, err := store.Open("sqlite.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Query for the specific entity
	var entity store.Entity

	err = db.Where("id = ?", entityID).First(&entity).Error
	if err != nil {
		return fmt.Errorf("%w: %s", errEntityNotFound, entityID)
	}

	// Pretty print the entity state as JSON
	return printEntityStateJSON(out, entity)
}

func printEntityStateJSON(out io.Writer, entity store.Entity) error {
	// Use prettyjson to format and colorize the output
	formatter := prettyjson.NewFormatter()
	formatter.Indent = 2

	coloredJSON, err := formatter.Marshal(entity.State)
	if err != nil {
		return fmt.Errorf("failed to format entity data: %w", err)
	}

	fmt.Fprintln(out, string(coloredJSON))

	return nil
}
