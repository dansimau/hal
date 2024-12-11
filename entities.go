package hal

import (
	"reflect"

	"github.com/dansimau/hal/homeassistant"
)

// EntityInterface is the interface that we accept which allows us to create
// custom components that embed an Entity.
type EntityInterface interface {
	ConnectionBinder

	GetID() string
	GetState() homeassistant.State
	SetState(event homeassistant.State)
}

type Entities []EntityInterface

// Entity is a base type for all entities that can be embedded into other types.
type Entity struct {
	connection *Connection
	state      homeassistant.State
}

func NewEntity(id string) *Entity {
	return &Entity{state: homeassistant.State{EntityID: id}}
}

// BindConnection binds the entity to the connection. This allows entities to
// publish messages.
func (e *Entity) BindConnection(connection *Connection) {
	e.connection = connection
}

func (e *Entity) GetID() string {
	return e.state.EntityID
}

func (e *Entity) SetState(state homeassistant.State) {
	e.state = state
}

func (e *Entity) GetState() homeassistant.State {
	return e.state
}

// findEntities recursively finds all entities in a struct, map, or slice.
func findEntities(v any) []EntityInterface {
	var entities []EntityInterface

	value := reflect.ValueOf(v)
	if value.Kind() == reflect.Ptr {
		value = value.Elem()
	}

	if value.Kind() != reflect.Struct {
		return entities
	}

	valueType := value.Type()

	for i := range value.NumField() {
		field := value.Field(i)
		fieldType := field.Type()

		// Skip unexported fields
		if !valueType.Field(i).IsExported() {
			continue
		}

		// Check if field implements EntityLike interface
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			if entity, ok := field.Interface().(EntityInterface); ok {
				entities = append(entities, entity)

				continue
			}
		}

		// Recursively check for nested structs, maps, and slices
		switch fieldType.Kind() {
		case reflect.Struct:
			if field.CanInterface() {
				entities = append(entities, findEntities(field.Interface())...)
			}
		case reflect.Ptr:
			if !field.IsNil() && field.CanInterface() {
				entities = append(entities, findEntities(field.Interface())...)
			}
		case reflect.Map:
			if !field.IsNil() && field.CanInterface() {
				for _, key := range field.MapKeys() {
					mapValue := field.MapIndex(key)
					if mapValue.CanInterface() {
						entities = append(entities, findEntities(mapValue.Interface())...)
					}
				}
			}
		case reflect.Slice:
			if !field.IsNil() && field.CanInterface() {
				for i := range field.Len() {
					sliceValue := field.Index(i)
					if sliceValue.CanInterface() {
						entities = append(entities, findEntities(sliceValue.Interface())...)
					}
				}
			}
		default:
			// Ignore other types
		}
	}

	return entities
}
