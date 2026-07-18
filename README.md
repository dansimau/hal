# Home Automation Logic (HAL) Framework

![checks](https://github.com/dansimau/hal/actions/workflows/checks.yaml/badge.svg)
![coverage](https://raw.githubusercontent.com/dansimau/hal/badges/.badges/main/coverage.svg)

HAL is a framework for programming home automation logic in Go on top of
[Home Assistant](https://www.home-assistant.io/).

## Key features

- 💻 **Automations as code.** Express triggers, conditions, and actions as plain
  Go functions with full access to the language.
- 🔌 **Typed entities.** Lights, binary sensors, light/lux sensors, buttons, and
  virtual switches (`input_boolean`) are strongly typed wrappers with convenient
  helpers (`IsOn()`, `TurnOn()`, `GetBrightness()`, `Level()`, ...).
- 💡 **Light groups.** Treat a set of lights as one; turn them on/off or set
  scenes together.
- 🔋 **Reusable, batteries-included automations.** Common patterns ship in the
  [`automations`](./automations) package — e.g. `SensorsTriggerLights`
  (motion/presence lights with dimming, cooldowns, human-override, and
  conditional scenes) and `Timer` (run an action after a debounced delay).
- 🎬 **Scenes.** Define reusable brightness/color presets as ordinary maps and
  apply them conditionally (e.g. a dim "night light" vs. a bright daytime scene).
- ☀️ **Sun & time awareness.** Built-in sunrise/sunset calculations
  (`IsDayTime()`, `IsNightTime()`, `Sunrise()`, `Sunset()`) based on your
  configured location.
- 🔒 **Ordered, race-free execution.** All state changes are serialized so
  automations fire in a predictable order.
- 💾 **State persistence & metrics.** Entity state and automation history are
  persisted to SQLite; timing/counter metrics are recorded automatically.
- 🔄 **Resilient connection.** Automatic reconnection with heartbeats to Home
  Assistant over its WebSocket API.
- 🛡️ **Loop protection.** State changes caused by HAL itself won't re-trigger
  your automations.
- 🧪 **Testable.** A `testutil` package plus a mockable clock let you unit-test
  time-dependent automations without waiting in real time.
- 🛠️ **Companion CLI.** The [`hal`](./cmd/hal) CLI inspects live entities, tails
  logs, shows metrics, streams events, and prunes old history.

## How it works

HAL connects to Home Assistant over its WebSocket API and subscribes to state
change events. You register:

1. **Entities** — typed handles to your Home Assistant devices, keyed by their
   entity ID (e.g. `light.kitchen`).
2. **Automations** — each declares the entities it listens on and an action to
   run. Whenever one of those entities changes state, the automation's action
   fires.

Entities can be discovered automatically by walking your home struct with
reflection (`FindEntities`), so you rarely have to register them by hand.

## Example

Here's a complete program: when the kitchen motion sensor trips, turn on the
light and turn it off again 15 minutes after the last movement.

```go
package main

import (
	"log/slog"
	"os"
	"time"

	"github.com/dansimau/hal"
	halautomations "github.com/dansimau/hal/automations"
)

func main() {
	// Reads config from hal.yaml
	cfg, err := hal.LoadConfig()
	if err != nil {
		slog.Error("Error loading config", "error", err)
		os.Exit(1)
	}
	conn := hal.NewConnection(*cfg)

	// Entities are typed handles to Home Assistant devices, keyed by entity ID.
	motion := hal.NewBinarySensor("binary_sensor.kitchen_motion")
	light := hal.NewLight("light.kitchen")

	// Register the entities so HAL tracks their state.
	conn.RegisterEntities(motion, light)

	// An automation declares the entities it listens on and what to do when one
	// of them changes. SensorsTriggerLights is a prebuilt helper that handles the
	// turn-on, the turn-off timer, and cooldowns for you.
	conn.RegisterAutomations(
		halautomations.NewSensorsTriggerLights().
			WithName("Kitchen light").
			WithSensors(motion).
			WithLights(light).
			TurnsOffAfter(15 * time.Minute),
	)

	if err := conn.Start(); err != nil {
		slog.Error("Error", "error", err)
		os.Exit(1)
	}
}
```

```sh
go run .
```

For anything a helper doesn't cover, write the logic yourself with
`NewAutomation()` (see [Building automations](#building-automations)). As your
setup grows, the idiomatic pattern is to model each room as a struct of typed
entities with an `Automations` method, then use `conn.FindEntities(home)` to
register every entity by walking the struct with reflection.


## Configuration

HAL looks for a `hal.yaml` file in the current directory or any parent directory:

```yaml
homeAssistant:
  host: homeassistant.local:8123
  # A long-lived access token from your Home Assistant profile.
  token: <your-long-lived-access-token>
  # The user ID the token belongs to. HAL uses this to ignore state changes
  # caused by its own actions, preventing automation loops.
  userId: <your-home-assistant-user-id>

# Your location, used for sunrise/sunset calculations.
location:
  lat: 51.5074
  lng: 0.1278

# Optional:
# databasePath: sqlite.db        # where to persist state (default: sqlite.db)
# reconnectInterval: 10s
# pingInterval: 30s
# readTimeout: 60s
```

## Entity types

| Type              | Constructor                     | Notable helpers                                                    |
| ----------------- | ------------------------------- | ------------------------------------------------------------------ |
| `Light`           | `hal.NewLight(id)`              | `IsOn()`, `TurnOn(attrs...)`, `TurnOff()`, `GetBrightness()`       |
| `LightGroup`      | `hal.LightGroup{...}`           | Same as `Light`, applied to every member; `IsOn()`, `IsOff()`      |
| `BinarySensor`    | `hal.NewBinarySensor(id)`       | `IsOn()`, `IsOff()`                                                |
| `LightSensor`     | `hal.NewLightSensor(id)`        | `Level()` (illuminance/lux as an int)                              |
| `InputBoolean`    | `hal.NewInputBoolean(id)`       | `IsOn()`, `IsOff()`, `TurnOn()`, `TurnOff()` (a virtual switch)    |
| `Button`          | `hal.NewButton(id)`             | `PressedTimes()` (detects multi-presses)                           |
| `Entity`          | `hal.NewEntity(id)`             | Base type: `GetID()`, `GetState()` for anything not yet typed      |

Every entity has `TurnOnContext` / `TurnOffContext` variants that thread a
`context.Context` through for tracing.

## Building automations

There are two ways to write automations:

**1. Inline, with `NewAutomation()`** — for bespoke logic:

```go
hal.NewAutomation().
	WithName("My automation").
	WithEntities(sensor). // fires whenever one of these entities changes
	WithAction(func(ctx context.Context, trigger hal.EntityInterface) {
		// ...your logic...
	})
```

**2. With a prebuilt helper from the [`automations`](./automations) package:**

- **`SensorsTriggerLights`** — the workhorse. Motion/presence sensors turn
  lights on and off after a delay, with optional dimming before turn-off,
  turn-off cooldown, conditional scenes, a run condition, and "human override"
  (backing off when someone changes the lights manually).
- **`Timer`** — run an action once its conditions have held for a set duration
  (with the timer resetting whenever a watched entity changes).
- **`PrintDebug`** — log state changes for a set of entities; handy while
  developing.

## Companion CLI

HAL ships with a CLI for inspecting a running deployment. Install it with:

```sh
go install github.com/dansimau/hal/cmd/hal@latest
```

It reads the same `hal.yaml`/SQLite database and provides:

| Command         | Purpose                                        |
| --------------- | ---------------------------------------------- |
| `hal entities`  | List entities and their current state          |
| `hal events`    | Stream live state-change events                |
| `hal logs`      | Tail automation logs                           |
| `hal stats`     | Show recorded metrics                          |
| `hal prune`     | Prune old history from the database            |

## Development

```sh
make test   # run tests with coverage
make lint   # run golangci-lint (installs it if missing)
```
