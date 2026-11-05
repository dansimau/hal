package hal

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/logger"
	"github.com/dansimau/hal/metrics"
	"github.com/dansimau/hal/perf"
	"github.com/dansimau/hal/store"
	"github.com/google/go-cmp/cmp"
)

// Connection is a new instance of the HAL framework. It connects to Home Assistant,
// listens for state updates and invokes automations when state changes are detected.
// TODO: Rename "Connection" to something more descriptive.
type Connection struct {
	*SunTimes

	config Config
	db     *store.Store

	automations map[string][]Automation
	entities    map[string]EntityInterface

	// timers are the durable timers registered by automations, keyed by the
	// name they are saved under. timersRestored records whether they have been
	// restored from the database yet; it is guarded by mutex.
	timers         map[string]registeredTimer
	timersRestored bool

	// Lock to serialize state updates and ensure automations fire in order.
	mutex sync.RWMutex

	homeAssistant  *hassws.Client
	metricsService *metrics.Service

	shutdownCh        chan struct{}
	closeOnce         sync.Once
	reconnectInterval time.Duration
	reconnectAttempts atomic.Int32
}

// parseFlushInterval parses a FlushIntervalEnv value: a non-negative Go
// duration.
func parseFlushInterval(value string) (time.Duration, error) {
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}

	if interval < 0 {
		return 0, fmt.Errorf("%w: %s", ErrNegativeFlushInterval, value)
	}

	return interval, nil
}

// ConnectionBinder is an interface that can be implemented by entities to bind
// them to a connection.
type ConnectionBinder interface {
	BindConnection(connection *Connection)
}

// FlushIntervalEnv is the environment variable that sets how long queued
// database writes (logs, metrics, entity states) are held before being
// committed together, as a Go duration such as "2s" or "500ms". Longer
// intervals mean fewer disk writes; shorter ones mean writes show up sooner in
// the hal CLI and less is lost if the process is killed. Defaults to
// store.DefaultFlushInterval.
const FlushIntervalEnv = "HAL_DB_FLUSH_INTERVAL"

// defaultReconnectInterval is how long to wait between reconnection attempts
// when Config.ReconnectInterval is unset.
const defaultReconnectInterval = 10 * time.Second

func NewConnection(cfg Config) *Connection {
	dbPath := cfg.DatabasePath
	if dbPath == "" {
		dbPath = "sqlite.db"
	}

	db, err := store.Open(dbPath)
	if err != nil {
		panic(err)
	}

	if value := os.Getenv(FlushIntervalEnv); value != "" {
		interval, parseErr := parseFlushInterval(value)
		if parseErr != nil {
			logger.Error("Ignoring invalid "+FlushIntervalEnv, "", "value", value, "error", parseErr)
		} else {
			db.SetFlushInterval(interval)
			logger.Info("Database flush interval set from "+FlushIntervalEnv, "", "interval", interval)
		}
	}

	api := hassws.NewClient(hassws.ClientConfig{
		Host:         cfg.HomeAssistant.Host,
		Token:        cfg.HomeAssistant.Token,
		PingInterval: cfg.PingInterval,
		ReadTimeout:  cfg.ReadTimeout,
	})

	// Set the database on the global logger
	logger.SetDefaultDatabase(db)

	// Set reconnect interval with default
	reconnectInterval := cfg.ReconnectInterval
	if reconnectInterval == 0 {
		reconnectInterval = defaultReconnectInterval
	}

	return &Connection{
		config:         cfg,
		db:             db,
		homeAssistant:  api,
		metricsService: metrics.NewService(db),

		automations: make(map[string][]Automation),
		entities:    make(map[string]EntityInterface),
		timers:      make(map[string]registeredTimer),

		SunTimes: NewSunTimes(cfg.Location),

		shutdownCh:        make(chan struct{}),
		reconnectInterval: reconnectInterval,
	}
}

func (h *Connection) CallService(msg hassws.CallServiceRequest) (hassws.CallServiceResponse, error) {
	return h.homeAssistant.CallService(msg)
}

// GetReconnectAttempts returns the number of reconnection attempts made.
func (h *Connection) GetReconnectAttempts() int {
	return int(h.reconnectAttempts.Load())
}

// ResetReconnectAttempts resets the reconnection attempt counter (useful for tests).
func (h *Connection) ResetReconnectAttempts() {
	h.reconnectAttempts.Store(0)
}

// FindEntities recursively finds and registers all entities in a struct, map, or slice.
func (h *Connection) FindEntities(v any) {
	h.RegisterEntities(findEntities(v)...)
}

// RegisterAutomations registers automations and binds them to the relevant
// entities. The timers of automations that implement TimerProvider are saved
// to the database and restored on the next start.
func (h *Connection) RegisterAutomations(automations ...Automation) {
	for _, automation := range automations {
		logger.Info("Registering automation", "", "Name", automation.Name())

		for _, entity := range automation.Entities() {
			h.automations[entity.GetID()] = append(h.automations[entity.GetID()], automation)
		}

		h.registerTimers(automation)
	}
}

// RegisterEntities registers entities and binds them to the connection.
func (h *Connection) RegisterEntities(entities ...EntityInterface) {
	for _, entity := range entities {
		entityID := entity.GetID()
		logger.Info("Registering entity", entityID)
		entity.BindConnection(h)
		h.entities[entity.GetID()] = entity

		// Entities can also be automations
		if automation, ok := entity.(Automation); ok {
			h.RegisterAutomations(automation)
		}
	}
}

// Start connects to the Home Assistant websocket and starts listening for events.
// This method is blocking and will retry connections indefinitely with 10-second intervals.
func (h *Connection) Start() error {
	// Start services
	h.metricsService.Start()
	logger.StartDefault()

	// Create disconnection signal channel
	disconnectedCh := make(chan struct{}, 1)

	// Set up disconnection callback
	h.homeAssistant.SetOnDisconnected(func() {
		select {
		case disconnectedCh <- struct{}{}:
		default:
			// Channel already has a signal, skip
		}
	})

	// Initial connection attempt
	err := h.connect()
	if err != nil {
		logger.Error("Initial connection failed", "", "error", err)
		// Signal reconnection to start retry loop
		disconnectedCh <- struct{}{}
	} else {
		logger.Info("Initial connection successful", "")
	}

	// Reconnection loop
	for {
		select {
		case <-disconnectedCh:
			logger.Warn("Connection lost, will retry", "", "interval", h.reconnectInterval)

			time.Sleep(h.reconnectInterval)

			h.reconnectAttempts.Add(1)
			attempt := h.GetReconnectAttempts()

			logger.Info("Attempting to reconnect", "", "attempt", attempt)

			err := h.connect()
			if err != nil {
				logger.Error("Reconnection failed", "", "error", err)
				// Signal reconnection for retry
				select {
				case disconnectedCh <- struct{}{}:
				default:
				}

				continue
			}

			logger.Info("Reconnection successful", "")

		case <-h.shutdownCh:
			logger.Info("Shutdown signal received, stopping reconnection loop", "")

			return nil
		}
	}
}

func (h *Connection) Close() {
	h.closeOnce.Do(func() {
		// Signal shutdown to stop reconnection loop
		close(h.shutdownCh)

		// Close WebSocket connection
		h.homeAssistant.Close()

		// Stop services
		h.metricsService.Stop()
		logger.StopDefault()

		// Save the state of every timer so running ones resume on the next
		// start, then flush pending database writes before closing.
		h.syncTimers()

		err := h.db.Close()
		if err != nil {
			logger.Error("Failed to close database", "", "error", err)
		}
	})
}

// StateChangeEvent processes incoming state change events. It dispatches the
// state change to the relevant entity and fires any automations listening for
// state changes to this entity.
func (h *Connection) StateChangeEvent(event hassws.EventMessage) {
	defer perf.Timer(func(timeTaken time.Duration) {
		logger.Debug("Tick processing time", event.Event.EventData.EntityID, "duration", timeTaken)
		// Record tick processing time metric
		h.metricsService.RecordTimer(store.MetricTypeTickProcessingTime, timeTaken, event.Event.EventData.EntityID, "")
	})()

	h.mutex.Lock()
	defer h.mutex.Unlock()

	entity, ok := h.entities[event.Event.EventData.EntityID]
	if !ok {
		logger.Debug("Entity not registered", event.Event.EventData.EntityID)

		return
	}

	logger.InfoDiff(
		"State changed for",
		event.Event.EventData.EntityID,
		cmp.Diff(event.Event.EventData.OldState, event.Event.EventData.NewState),
	)

	// A nil NewState carries no state to apply (e.g. entity removed); leave the
	// stored state untouched rather than overwriting it with nil.
	if event.Event.EventData.NewState == nil {
		return
	}

	newState := *event.Event.EventData.NewState

	// Drop stale updates that arrive out of order. Home Assistant events can be
	// reordered in transit, and a reconnect resync can race with live events; an
	// older state must never overwrite a newer one. Equal timestamps still apply.
	current := entity.GetState()
	if !current.LastUpdated.IsZero() && newState.LastUpdated.Before(current.LastUpdated) {
		logger.Debug("Skipping stale state update", event.Event.EventData.EntityID)

		return
	}

	entity.SetState(newState)

	// Update database asynchronously
	entityID := event.Event.EventData.EntityID
	h.db.EnqueueEntityState(entityID, &newState)

	// Prevent loops by not running automations that originate from hal
	if event.Event.Context.UserID == h.config.HomeAssistant.UserID {
		logger.Debug("Skipping automation from own action", event.Event.EventData.EntityID)

		return
	}

	// Dispatch automations
	for _, automation := range h.automations[event.Event.EventData.EntityID] {
		// Create context with tracing metadata
		ctx := NewAutomationContext(event.Event.EventData.EntityID, automation.Name())

		logger.InfoContext(ctx, "Running automation")
		// Record automation triggered metric
		h.metricsService.RecordCounter(store.MetricTypeAutomationTriggered, event.Event.EventData.EntityID, automation.Name())
		automation.Action(ctx, entity)
	}
}

// connect establishes the WebSocket connection, subscribes to events, syncs
// states and, on the first connection, restores saved timers.
func (h *Connection) connect() error {
	err := h.homeAssistant.Connect()
	if err != nil {
		return err
	}

	// Hold the state lock from subscribing until timers are restored, so no
	// event reaches an automation before its timers are back in place: an
	// event could otherwise cancel a timer that is not running yet, only for
	// the restore to start it afterwards. Events arriving meanwhile are
	// buffered by the client and dispatched, in order, once the lock is
	// released.
	h.mutex.Lock()
	defer h.mutex.Unlock()

	err = h.homeAssistant.SubscribeEvents(string(hassws.MessageTypeStateChanged), h.StateChangeEvent)
	if err != nil {
		return fmt.Errorf("failed to subscribe to state changed events: %w", err)
	}

	err = h.syncStates()
	if err != nil {
		return fmt.Errorf("failed to sync initial states: %w", err)
	}

	h.restoreTimers()

	return nil
}

func (h *Connection) syncStates() error {
	defer perf.Timer(func(timeTaken time.Duration) {
		logger.Info("Initial state sync complete", "", "duration", timeTaken)
	})()

	states, err := h.homeAssistant.GetStates()
	if err != nil {
		return err
	}

	for _, state := range states {
		entity, ok := h.entities[state.EntityID]
		if !ok {
			continue
		}

		logger.Debug("Setting initial state", state.EntityID, "State", state)

		entity.SetState(state)
	}

	return nil
}
