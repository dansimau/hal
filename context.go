package hal

import (
	"context"

	"github.com/dansimau/hal/logger"
)

// NewAutomationContext creates a context with automation metadata.
//
// Values are stored under the keys defined in the logger package so that the
// context-aware loggers (logger.InfoContext, etc.) can read the entity ID and
// automation name back. These keys must be shared: context.Value compares keys
// by dynamic type as well as value, so a key defined in this package would not
// match one defined in the logger package even with the same underlying string.
func NewAutomationContext(triggerEntityID string, automationName string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, logger.EntityIDKey, triggerEntityID)
	ctx = context.WithValue(ctx, logger.AutomationNameKey, automationName)
	return ctx
}

// GetEntityIDFromContext extracts the entity ID from context
func GetEntityIDFromContext(ctx context.Context) string {
	if entityID, ok := ctx.Value(logger.EntityIDKey).(string); ok {
		return entityID
	}
	return ""
}

// GetAutomationNameFromContext extracts the automation name from context
func GetAutomationNameFromContext(ctx context.Context) string {
	if name, ok := ctx.Value(logger.AutomationNameKey).(string); ok {
		return name
	}
	return ""
}
