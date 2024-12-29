package halautomations

import (
	"log"

	"github.com/dansimau/hal"
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

func (p *PrintDebug) Action(_ hal.EntityInterface) {
	for _, entity := range p.entities {
		log.Printf("[%s] Entity %s state: %+v", p.name, entity.GetID(), entity.GetState())
	}
}
