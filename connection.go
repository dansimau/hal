package hal

import (
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/metrics"
	"github.com/dansimau/hal/perf"
	"github.com/dansimau/hal/store"
	"github.com/google/go-cmp/cmp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Connection is a new instance of the HAL framework. It connects to Home Assistant,
// listens for state updates and invokes automations when state changes are detected.
// TODO: Rename "Connection" to something more descriptive.
type Connection struct {
	config Config
	db     *gorm.DB

	automations map[string][]Automation
	entities    map[string]EntityInterface

	// Lock to serialize state updates and ensure automations fire in order.
	mutex sync.RWMutex

	homeAssistant *hassws.Client
	metricsService *metrics.Service

	*SunTimes
}

// ConnectionBinder is an interface that can be implemented by entities to bind
// them to a connection.
type ConnectionBinder interface {
	BindConnection(connection *Connection)
}

func NewConnection(cfg Config) *Connection {
	dbPath := cfg.DatabasePath
	if dbPath == "" {
		dbPath = "sqlite.db"
	}

	db, err := store.Open(dbPath)
	if err != nil {
		panic(err)
	}

	api := hassws.NewClient(hassws.ClientConfig{
		Host:  cfg.HomeAssistant.Host,
		Token: cfg.HomeAssistant.Token,
	})

	return &Connection{
		config:         cfg,
		db:             db,
		homeAssistant:  api,
		metricsService: metrics.NewService(db),

		automations: make(map[string][]Automation),
		entities:    make(map[string]EntityInterface),

		SunTimes: NewSunTimes(cfg.Location),
	}
}

func (h *Connection) CallService(msg hassws.CallServiceRequest) (hassws.CallServiceResponse, error) {
	return h.homeAssistant.CallService(msg)
}

// FindEntities recursively finds and registers all entities in a struct, map, or slice.
func (h *Connection) FindEntities(v any) {
	h.RegisterEntities(findEntities(v)...)
}

// RegisterAutomations registers automations and binds them to the relevant entities.
func (h *Connection) RegisterAutomations(automations ...Automation) {
	for _, automation := range automations {
		slog.Info("Registering automation", "Name", automation.Name())

		for _, entity := range automation.Entities() {
			h.automations[entity.GetID()] = append(h.automations[entity.GetID()], automation)
		}
	}
}

// RegisterEntities registers entities and binds them to the connection.
func (h *Connection) RegisterEntities(entities ...EntityInterface) {
	for _, entity := range entities {
		slog.Info("Registering entity", "EntityID", entity.GetID())
		entity.BindConnection(h)
		h.entities[entity.GetID()] = entity

		// Entities can also be automations
		if automation, ok := entity.(Automation); ok {
			h.RegisterAutomations(automation)
		}
	}
}

// Start connects to the Home Assistant websocket and starts listening for events.
func (h *Connection) Start() error {
	// Start metrics service
	h.metricsService.Start()

	if err := h.homeAssistant.Connect(); err != nil {
		return err
	}

	if err := h.homeAssistant.SubscribeEvents(string(hassws.MessageTypeStateChanged), h.StateChangeEvent); err != nil {
		return fmt.Errorf("failed to subscribe to state changed events: %w", err)
	}

	if err := h.syncStates(); err != nil {
		return fmt.Errorf("failed to sync initial states: %w", err)
	}

	return nil
}

func (h *Connection) Close() {
	h.metricsService.Stop()
	h.homeAssistant.Close()
}

func (h *Connection) syncStates() error {
	defer perf.Timer(func(timeTaken time.Duration) {
		slog.Info("Initial state sync complete", "duration", timeTaken)
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

		slog.Debug("Setting initial state", "EntityID", state.EntityID, "State", state)

		entity.SetState(state)
	}

	return nil
}

// Process incoming state change events. Dispatch state change to the relevant
// entity and fire any automations listening for state changes to this entity.
func (h *Connection) StateChangeEvent(event hassws.EventMessage) {
	defer perf.Timer(func(timeTaken time.Duration) {
		slog.Debug("Tick processing time", "duration", timeTaken)
		// Record tick processing time metric
		h.metricsService.RecordTimer(store.MetricTypeTickProcessingTime, timeTaken, event.Event.EventData.EntityID, "")
	})()

	// Record automation evaluation metric
	h.metricsService.RecordCounter(store.MetricTypeAutomationEvaluated, event.Event.EventData.EntityID, "")

	h.mutex.Lock()
	defer h.mutex.Unlock()

	entity, ok := h.entities[event.Event.EventData.EntityID]
	if !ok {
		slog.Debug("Entity not registered", "EntityID", event.Event.EventData.EntityID)

		return
	}

	slog.Debug("State changed for",
		"EntityID", event.Event.EventData.EntityID,
	)

	fmt.Fprintf(os.Stderr, "Diff:\n%s\n", cmp.Diff(event.Event.EventData.OldState, event.Event.EventData.NewState))

	if event.Event.EventData.NewState != nil {
		entity.SetState(*event.Event.EventData.NewState)
	}

	// Update database
	h.db.Clauses(clause.OnConflict{
		UpdateAll: true,
	}).Create(&store.Entity{
		ID:    event.Event.EventData.EntityID,
		Type:  entity.GetID(),
		State: event.Event.EventData.NewState,
	})

	// Prevent loops by not running automations that originate from hal
	if event.Event.Context.UserID == h.config.HomeAssistant.UserID {
		slog.Debug("Skipping automation from own action", "EntityID", event.Event.EventData.EntityID)

		return
	}

	// Dispatch automations
	for _, automation := range h.automations[event.Event.EventData.EntityID] {
		slog.Info("Running automation", "name", automation.Name())
		// Record automation triggered metric
		h.metricsService.RecordCounter(store.MetricTypeAutomationTriggered, event.Event.EventData.EntityID, automation.Name())
		automation.Action(entity)
	}
}
