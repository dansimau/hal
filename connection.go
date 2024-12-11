package hal

import (
	"log/slog"
	"sync"
	"time"

	"github.com/dansimau/hal/hassws"
	"github.com/dansimau/hal/perf"
	"github.com/davecgh/go-spew/spew"
)

// Connection is a new instance of the HAL framework. It connects to Home Assistant,
// listens for state updates and invokes automations when state changes are detected.
// TODO: Rename "Connection" to something more descriptive.
type Connection struct {
	config Config

	automations map[string][]Automation
	entities    map[string]EntityInterface

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
	api := hassws.NewClient(hassws.ClientConfig{
		Host:  cfg.HomeAssistant.Host,
		Token: cfg.HomeAssistant.Token,
	})

	return &Connection{
		config:        cfg,
		homeAssistant: api,

		automations: make(map[string][]Automation),
		entities:    make(map[string]EntityInterface),

		SunTimes: NewSunTimes(cfg.Location),
	}
}

// HomeAssistant returns the underlying Home Assistant websocket client.
func (h *Connection) HomeAssistant() *hassws.Client {
	return h.homeAssistant
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
	}
}

// Start connects to the Home Assistant websocket and starts listening for events.
func (h *Connection) Start() error {
	if err := h.HomeAssistant().Connect(); err != nil {
		return err
	}

	return h.HomeAssistant().SubscribeEvents(string(hassws.MessageTypeStateChanged), h.StateChangeEvent)
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

	// Dispatch automations
	for _, automation := range h.automations[event.Event.EventData.EntityID] {
		slog.Info("Running automation", "name", automation.Name())
		automation.Action()
	}
}
