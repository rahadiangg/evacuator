package evacuator

import (
	"log/slog"
	"os"
	"testing"
)

// Mock handler that always fails to create
type mockFailingHandler struct{}

func (m *mockFailingHandler) HandleTermination(ctx interface{}, event TerminationEvent) error {
	return nil
}

func (m *mockFailingHandler) Name() string {
	return "mock-failing"
}

func TestHandlerRegistry_NilHandlerNotAppended(t *testing.T) {
	// Create a handler registry
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewHandlerRegistry(logger)

	// Set up config to enable kubernetes but make it fail
	// This test verifies that when handler creation fails, nil is not appended
	SetGlobalConfig(&Config{
		Provider: ProviderConfig{
			Name: "aws", // Use non-dummy provider to avoid dummy handler
		},
		Handler: HandlerConfig{
			Kubernetes: KubernetesConfig{
				Enabled:    true,
				InCluster:  true,
				Kubeconfig: "",
			},
		},
	})

	// This will fail because we're not actually in a cluster
	handlers, err := registry.RegisterHandlers()

	// Verify we got an error (since kubernetes handler creation should fail and no handlers were registered)
	if err == nil {
		t.Error("expected error from handler creation, got nil")
	}

	// Verify that no handlers were registered (dummy handler should not be added)
	if len(handlers) != 0 {
		t.Errorf("expected 0 handlers, got %d", len(handlers))
	}

	// Verify that no nil handlers were appended
	for i, h := range handlers {
		if h == nil {
			t.Errorf("handler at index %d is nil (should not be appended on creation failure)", i)
		}
	}

	// Reset global config
	globalConfig = nil
}

func TestHandlerRegistry_DummyHandlerRegistered(t *testing.T) {
	// Create a handler registry
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewHandlerRegistry(logger)

	// Set up config with dummy provider
	SetGlobalConfig(&Config{
		Provider: ProviderConfig{
			Name: "dummy",
		},
		Handler: HandlerConfig{
			ProcessingTimeout: 30,
		},
	})

	handlers, err := registry.RegisterHandlers()

	// Verify no errors
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify dummy handler was registered
	if len(handlers) != 1 {
		t.Errorf("expected 1 handler, got %d", len(handlers))
	}

	if len(handlers) > 0 && handlers[0] == nil {
		t.Error("dummy handler is nil")
	}

	if len(handlers) > 0 && handlers[0].Name() != "dummy" {
		t.Errorf("expected handler name 'dummy', got %s", handlers[0].Name())
	}

	// Reset global config
	globalConfig = nil
}

func TestHandlerRegistry_NoHandlersRegistered(t *testing.T) {
	// Create a handler registry
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewHandlerRegistry(logger)

	// Set up config with no handlers enabled and non-dummy provider
	SetGlobalConfig(&Config{
		Provider: ProviderConfig{
			Name: "aws",
		},
		Handler: HandlerConfig{
			ProcessingTimeout: 30,
		},
	})

	handlers, err := registry.RegisterHandlers()

	// Verify we got an error about no handlers
	if len(handlers) != 0 {
		t.Errorf("expected 0 handlers, got %d", len(handlers))
	}

	if err == nil {
		t.Error("expected error about no handlers registered")
	}

	// Reset global config
	globalConfig = nil
}

func TestHandlerRegistry_MultipleHandlersRegistered(t *testing.T) {
	// Create a handler registry
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	registry := NewHandlerRegistry(logger)

	// Set up config with dummy provider (which enables dummy handler)
	SetGlobalConfig(&Config{
		Provider: ProviderConfig{
			Name: "dummy",
		},
		Handler: HandlerConfig{
			ProcessingTimeout: 30,
		},
	})

	handlers, err := registry.RegisterHandlers()

	// Verify no errors
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify at least dummy handler was registered
	if len(handlers) == 0 {
		t.Error("expected at least 1 handler, got 0")
	}

	// Verify no nil handlers in the list
	for i, h := range handlers {
		if h == nil {
			t.Errorf("handler at index %d is nil", i)
		}
		if h.Name() == "" {
			t.Errorf("handler at index %d has empty name", i)
		}
	}

	// Reset global config
	globalConfig = nil
}

func TestTerminationEvent(t *testing.T) {
	tests := []struct {
		name   string
		event  TerminationEvent
		valid  bool
	}{
		{
			name: "valid event with all fields",
			event: TerminationEvent{
				Hostname:   "test-host",
				PrivateIP:  "10.0.0.1",
				InstanceID: "i-123456",
				Reason:     TerminationReasonSpot,
			},
			valid: true,
		},
		{
			name: "event with unknown fields",
			event: TerminationEvent{
				Hostname:   "unknown",
				PrivateIP:  "unknown",
				InstanceID: "unknown",
				Reason:     TerminationReasonSpot,
			},
			valid: true,
		},
		{
			name: "event with maintenance reason",
			event: TerminationEvent{
				Hostname:   "test-host",
				PrivateIP:  "10.0.0.1",
				InstanceID: "i-123456",
				Reason:     TerminationReasonMaintenance,
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify the event can be created and fields are accessible
			if tt.event.Hostname == "" && tt.valid {
				t.Error("expected hostname to be set")
			}
			if tt.event.Reason == "" && tt.valid {
				t.Error("expected reason to be set")
			}
		})
	}
}
