package hal

import (
	"context"
	"testing"
)

func TestNewAutomationContext(t *testing.T) {
	entityID := "light.kitchen"
	automationName := "test_automation"

	ctx := NewAutomationContext(entityID, automationName)

	// Test entity ID extraction
	extractedEntityID := GetEntityIDFromContext(ctx)
	if extractedEntityID != entityID {
		t.Errorf("Expected entity ID %q, got %q", entityID, extractedEntityID)
	}

	// Test automation name extraction
	extractedAutomationName := GetAutomationNameFromContext(ctx)
	if extractedAutomationName != automationName {
		t.Errorf("Expected automation name %q, got %q", automationName, extractedAutomationName)
	}
}

func TestGetEntityIDFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	entityID := GetEntityIDFromContext(ctx)
	if entityID != "" {
		t.Errorf("Expected empty entity ID, got %q", entityID)
	}
}

func TestGetAutomationNameFromContext_Empty(t *testing.T) {
	ctx := context.Background()

	automationName := GetAutomationNameFromContext(ctx)
	if automationName != "" {
		t.Errorf("Expected empty automation name, got %q", automationName)
	}
}

func TestContextImmutable(t *testing.T) {
	entityID := "light.kitchen"
	automationName := "test_automation"

	ctx := NewAutomationContext(entityID, automationName)

	// Create a new context with different values
	newEntityID := "sensor.motion"
	newAutomationName := "another_automation"
	newCtx := NewAutomationContext(newEntityID, newAutomationName)

	// Verify original context is unchanged
	if GetEntityIDFromContext(ctx) != entityID {
		t.Error("Original context entity ID was modified")
	}
	if GetAutomationNameFromContext(ctx) != automationName {
		t.Error("Original context automation name was modified")
	}

	// Verify new context has correct values
	if GetEntityIDFromContext(newCtx) != newEntityID {
		t.Error("New context entity ID is incorrect")
	}
	if GetAutomationNameFromContext(newCtx) != newAutomationName {
		t.Error("New context automation name is incorrect")
	}
}
