package hal_test

import (
	"context"
	"testing"

	"github.com/dansimau/hal"
)

func TestNewAutomationContext(t *testing.T) {
	t.Parallel()

	entityID := "light.kitchen"
	automationName := "test_automation"

	ctx := hal.NewAutomationContext(entityID, automationName)

	// Test entity ID extraction
	extractedEntityID := hal.GetEntityIDFromContext(ctx)
	if extractedEntityID != entityID {
		t.Errorf("Expected entity ID %q, got %q", entityID, extractedEntityID)
	}

	// Test automation name extraction
	extractedAutomationName := hal.GetAutomationNameFromContext(ctx)
	if extractedAutomationName != automationName {
		t.Errorf("Expected automation name %q, got %q", automationName, extractedAutomationName)
	}
}

func TestGetEntityIDFromContext_Empty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	entityID := hal.GetEntityIDFromContext(ctx)
	if entityID != "" {
		t.Errorf("Expected empty entity ID, got %q", entityID)
	}
}

func TestGetAutomationNameFromContext_Empty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	automationName := hal.GetAutomationNameFromContext(ctx)
	if automationName != "" {
		t.Errorf("Expected empty automation name, got %q", automationName)
	}
}

func TestContextImmutable(t *testing.T) {
	t.Parallel()

	entityID := "light.kitchen"
	automationName := "test_automation"

	ctx := hal.NewAutomationContext(entityID, automationName)

	// Create a new context with different values
	newEntityID := "sensor.motion"
	newAutomationName := "another_automation"
	newCtx := hal.NewAutomationContext(newEntityID, newAutomationName)

	// Verify original context is unchanged
	if hal.GetEntityIDFromContext(ctx) != entityID {
		t.Error("Original context entity ID was modified")
	}

	if hal.GetAutomationNameFromContext(ctx) != automationName {
		t.Error("Original context automation name was modified")
	}

	// Verify new context has correct values
	if hal.GetEntityIDFromContext(newCtx) != newEntityID {
		t.Error("New context entity ID is incorrect")
	}

	if hal.GetAutomationNameFromContext(newCtx) != newAutomationName {
		t.Error("New context automation name is incorrect")
	}
}
