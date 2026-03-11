package halautomations

import (
	"context"

	"github.com/dansimau/hal"
	"github.com/dansimau/hal/logger"
)

// PrintDebug prints state changes for the specified entities.
type PrintDebug struct {
	name     string
	entities hal.Entities
}

func NewPrintDebug(name string, entities ...hal.EntityInterface) *PrintDebug {
	return &PrintDebug{name: name, entities: entities}
}

func (p *PrintDebug) Name() string {
	return p.name
}

func (p *PrintDebug) Entities() hal.Entities {
	return p.entities
}

func (p *PrintDebug) Action(ctx context.Context, _ hal.EntityInterface) {
	for _, entity := range p.entities {
		logger.InfoContext(ctx, "Entity state", "entity", entity.GetID(), "state", entity.GetState())
	}
}
