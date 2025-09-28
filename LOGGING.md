# HAL Logging System

This document describes the logging system implemented for HAL that stores logs both to the console and SQLite database.

## Features

- **Dual logging**: All logs are written to both console (using slog) and SQLite database
- **Entity tracking**: Logs can be associated with specific entity IDs
- **Database storage**: Logs are stored with timestamp, optional entity ID, and log text
- **CLI viewing**: View logs through the `hal logs` command with various filtering options
- **Automatic pruning**: Old logs are automatically removed (keeps 1 month of logs)
- **Indexed fields**: Timestamp and entity_id are indexed for fast queries

## Database Schema

The `logs` table contains:
- `id` (primary key, auto-increment)
- `timestamp` (indexed, not null)
- `entity_id` (indexed, nullable, max 100 chars)
- `log_text` (not null, text field)

## Usage

### Programmatic Logging

```go
import (
    "github.com/dansimau/hal/logging"
    "github.com/dansimau/hal/store"
)

// Create logging service
db, err := store.Open("sqlite.db")
if err != nil {
    // handle error
}
logger := logging.NewService(db)

// Start the service (enables background pruning)
logger.Start()
defer logger.Stop()

// Log without entity ID
logger.Info("Application started", nil)
logger.Error("Connection failed", nil)

// Log with entity ID
logger.InfoWithEntity("Motion detected", "sensor.living_room_motion")
logger.WarnWithEntity("Low battery", "sensor.door_battery")

// Or use the generic methods
entityID := "light.living_room"
logger.Info("Light turned on", &entityID)
```

### Integration Example

To integrate into the main HAL Connection struct (similar to metrics service):

```go
type Connection struct {
    // ... existing fields ...
    metricsService *metrics.Service
    loggingService *logging.Service  // Add this
}

// In the constructor:
func NewConnection(cfg *Config) (*Connection, error) {
    // ... existing setup ...
    
    return &Connection{
        // ... existing fields ...
        metricsService: metrics.NewService(db),
        loggingService: logging.NewService(db),  // Add this
    }, nil
}

// Start services:
func (c *Connection) Start() {
    c.metricsService.Start()
    c.loggingService.Start()  // Add this
}
```

### CLI Commands

View all logs:
```bash
hal logs
```

Filter by date range:
```bash
hal logs --from "2023-12-01" --to "2023-12-31"
```

View recent logs:
```bash
hal logs --last 5m    # Last 5 minutes
hal logs --last 1h    # Last hour
hal logs --last 1d    # Last day
```

Filter by entity:
```bash
hal logs --entity-id sensor.living_room_motion
```

Use custom database:
```bash
hal logs --db /path/to/custom.db
```

## Log Levels

The logging service supports all standard log levels:
- `Debug()` / `DebugWithEntity()`
- `Info()` / `InfoWithEntity()`
- `Warn()` / `WarnWithEntity()`
- `Error()` / `ErrorWithEntity()`

All log methods accept:
1. `msg string` - The log message
2. `entityID *string` or `entityID string` - Optional entity ID
3. `args ...any` - Additional structured logging arguments (passed to slog)

## Automatic Maintenance

- **Pruning**: Logs older than 1 month are automatically deleted daily
- **Performance**: Uses SQLite WAL mode for efficient concurrent writes
- **Indexing**: Timestamp and entity_id fields are indexed for fast queries

## Time Specifications

For the `--last` flag, supported time units are:
- `s` - seconds
- `m` - minutes  
- `h` - hours
- `d` - days

Examples: `5m`, `1h`, `2d`, `30s`

## Date Formats

For `--from` and `--to` flags, supported formats include:
- `2023-12-01`
- `2023-12-01 15:04:05`
- `2023-12-01T15:04:05`
- RFC3339 format

## Testing

Run the logging tests:
```bash
go test ./logging
```

The tests verify:
- Log storage to database
- Console output
- Entity ID handling
- Log pruning functionality