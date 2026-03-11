package hal

import "context"

type contextKey string

const (
	// EntityIDKey is the context key for storing the triggering entity ID
	EntityIDKey contextKey = "entity_id"

	// AutomationNameKey is the context key for storing the automation name
	AutomationNameKey contextKey = "automation_name"
)

// NewAutomationContext creates a context with automation metadata
func NewAutomationContext(triggerEntityID string, automationName string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, EntityIDKey, triggerEntityID)
	ctx = context.WithValue(ctx, AutomationNameKey, automationName)
	return ctx
}

// GetEntityIDFromContext extracts the entity ID from context
func GetEntityIDFromContext(ctx context.Context) string {
	if entityID, ok := ctx.Value(EntityIDKey).(string); ok {
		return entityID
	}
	return ""
}

// GetAutomationNameFromContext extracts the automation name from context
func GetAutomationNameFromContext(ctx context.Context) string {
	if name, ok := ctx.Value(AutomationNameKey).(string); ok {
		return name
	}
	return ""
}
