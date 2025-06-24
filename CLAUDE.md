# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Common Development Commands

- **Build and Test**: `make test` - Runs tests with coverage reporting
- **Lint**: `make lint` - Runs golangci-lint (installs if missing)
- **Run Tests**: `go test -v ./...` - Run all tests with verbose output
- **Run Single Test**: `go test -v ./[package] -run TestFunctionName`
- **Coverage**: Tests automatically generate coverage report in `cover.out`

## Architecture Overview

HAL is a Go-based home automation framework that connects to Home Assistant via WebSocket and executes user-defined automations based on entity state changes.

### Core Components

- **Connection** (`connection.go`): Main framework controller that manages WebSocket connection to Home Assistant, maintains entity state, and orchestrates automation execution
- **Automations** (`automations.go`): Interface and builder pattern for creating automations that respond to entity state changes
- **Entities** (`entity_*.go`): Typed wrappers for Home Assistant entities (lights, sensors, buttons, etc.) with state management
- **WebSocket Client** (`hassws/`): Low-level Home Assistant WebSocket API client for real-time communication
- **State Store** (`store/`): SQLite-based persistence layer for entity states and automation history
- **Configuration** (`config.go`): YAML-based configuration system that searches parent directories for `hal.yaml`

### Key Patterns

- Automations use builder pattern: `NewAutomation().WithEntities(...).WithAction(...)`
- All entity state changes are serialized through mutex to ensure automations fire in order
- Entities implement `EntityInterface` and can bind to connections for Home Assistant integration
- State persistence uses GORM with SQLite backend
- Sun/solar calculations available via embedded `SunTimes` in Connection

### Testing

- Uses `testutil/` package with mocking utilities and wait helpers
- Clock mocking available via `github.com/benbjohnson/clock` 
- Test coverage enforcement via `.testcoverage.yaml` configuration
- Integration tests mock WebSocket connections to Home Assistant