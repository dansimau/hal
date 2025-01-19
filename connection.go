package hal

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/perf"
	"github.com/dansimau/hal/store"
	"github.com/davecgh/go-spew/spew"
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

	// Map of entity IDs to the number of expected state updates. Automations don't fire unless the expected updates are
	// 0. This is loop protection to prevent automations from triggering themselves.
	expectedStateUpdates map[string]*atomic.Int32

	// Lock to serialize state updates and ensure automations fire in order.
	mutex sync.RWMutex

	homeAssistant *hassws.Client

	*SunTimes
}

// ConnectionBinder is an interface that can be implemented by entities to bind
// them to a connection.
type ConnectionBinder interface {
	BindConnection(connection *Connection)
}

func NewConnection(cfg Config) *Connection {
	db, err := store.Open("sqlite.db")
	if err != nil {
		panic(err)
	}

	api := hassws.NewClient(hassws.ClientConfig{
		Host:  cfg.HomeAssistant.Host,
		Token: cfg.HomeAssistant.Token,
	})

	return &Connection{
		config:        cfg,
		db:            db,
		homeAssistant: api,

		automations:          make(map[string][]Automation),
		entities:             make(map[string]EntityInterface),
		expectedStateUpdates: make(map[string]*atomic.Int32),

		SunTimes: NewSunTimes(cfg.Location),
	}
}

func (h *Connection) CallService(msg hassws.CallServiceRequest) (hassws.CallServiceResponse, error) {
	entityID := msg.Data["entity_id"]

	entityIDs := getStringOrStringSlice(entityID)
	if len(entityIDs) == 0 {
		slog.Warn("No entity_id in call service request", "msg", msg)
	}

	for _, entityID := range entityIDs {
		c := h.expectedStateUpdatesForEntity(entityID).Add(1)
		slog.Info("Incremented expected state updates counter", "EntityID", entityID, "Count", c)
	}

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
	h.homeAssistant.Close()
}

func (h *Connection) expectedStateUpdatesForEntity(entityID string) *atomic.Int32 {
	if _, ok := h.expectedStateUpdates[entityID]; !ok {
		h.expectedStateUpdates[entityID] = &atomic.Int32{}
	}

	return h.expectedStateUpdates[entityID]
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
	})()

	h.mutex.Lock()
	defer h.mutex.Unlock()

	entity, ok := h.entities[event.Event.EventData.EntityID]
	if !ok {
		slog.Debug("Entity not registered", "EntityID", event.Event.EventData.EntityID)

		return
	}

	slog.Debug("State changed for",
		"EntityID", event.Event.EventData.EntityID,
		"NewState", spew.Sdump(event.Event.EventData.NewState),
	)

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

	// Prevent automation loops by decrementing the expected state updates counter. Note that this approach is not
	// foolproof. If multiple state updates happen at the same time, the automation may be triggered on a state change
	// that is not expected.
	expectedStateUpdates := h.expectedStateUpdatesForEntity(event.Event.EventData.EntityID)
	if expectedStateUpdates.Load() > 0 {
		expectedStateUpdates.Add(-1)

		slog.Info("Skipping automations to prevent loop", "EntityID", event.Event.EventData.EntityID)

		return
	}

	// Dispatch automations
	for _, automation := range h.automations[event.Event.EventData.EntityID] {
		slog.Info("Running automation", "name", automation.Name())
		automation.Action(entity)
	}
}
