package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rahadiangg/evacuator"
)

// mockHandler is a test handler that records events it receives
type mockHandler struct {
	name   string
	mu     sync.Mutex
	events []evacuator.TerminationEvent
	delays []time.Duration // Simulate processing delays
}

func newMockHandler(name string, delays ...time.Duration) *mockHandler {
	return &mockHandler{
		name:   name,
		events: make([]evacuator.TerminationEvent, 0),
		delays: delays,
	}
}

func (m *mockHandler) HandleTermination(ctx context.Context, event evacuator.TerminationEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Simulate processing delay if configured
	if len(m.delays) > 0 {
		delay := m.delays[0]
		if len(m.delays) > 1 {
			m.delays = m.delays[1:]
		}
		if delay > 0 {
			time.Sleep(delay)
		}
	}

	m.events = append(m.events, event)
	return nil
}

func (m *mockHandler) Name() string {
	return m.name
}

func (m *mockHandler) getEvents() []evacuator.TerminationEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.events
}

func TestEventCopyPreventsRace(t *testing.T) {
	// This test verifies that each handler gets its own copy of the event
	// and modifications in one handler don't affect others

	// Create handlers with different delays to maximize chance of race
	handlers := []evacuator.Handler{
		newMockHandler("handler1", 10*time.Millisecond),
		newMockHandler("handler2", 5*time.Millisecond),
		newMockHandler("handler3", 0), // No delay
	}

	// Create a test event
	originalEvent := evacuator.TerminationEvent{
		Hostname:   "original-host",
		PrivateIP:  "10.0.0.1",
		InstanceID: "i-123456",
		Reason:     evacuator.TerminationReasonSpot,
	}

	// Simulate the broadcast pattern from main.go
	ctx := context.Background()
	config := &evacuator.Config{
		NodeName: "modified-host", // This would be applied to each handler's event copy
	}
	evacuator.SetGlobalConfig(config)
	defer func() { evacuator.SetGlobalConfig(nil) }()

	var handlerWg sync.WaitGroup
	results := make(chan HandlerResult, len(handlers))

	for _, handler := range handlers {
		handler := handler // Capture loop variable
		handlerWg.Add(1)
		go func() {
			defer handlerWg.Done()

			// Create a copy of the event for this handler (as fixed in main.go)
			handlerEvent := originalEvent
			if config.NodeName != "" {
				handlerEvent.Hostname = config.NodeName
			}

			handlerCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer cancel()

			err := handler.HandleTermination(handlerCtx, handlerEvent)
			results <- HandlerResult{
				HandlerName: handler.Name(),
				Error:       err,
				ProcessedAt: time.Now(),
			}
		}()
	}

	handlerWg.Wait()
	close(results)

	// Collect results
	successCount := 0
	for result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	if successCount != len(handlers) {
		t.Errorf("expected %d successful handlers, got %d", len(handlers), successCount)
	}

	// Verify each handler received the correct (modified) hostname
	for _, h := range handlers {
		mockH := h.(*mockHandler)
		events := mockH.getEvents()
		if len(events) != 1 {
			t.Errorf("handler %s expected 1 event, got %d", mockH.Name(), len(events))
			continue
		}
		if events[0].Hostname != "modified-host" {
			t.Errorf("handler %s expected hostname 'modified-host', got '%s'", mockH.Name(), events[0].Hostname)
		}
	}
}

func TestEventConcurrentModification(t *testing.T) {
	// This test verifies concurrent modification of the same event doesn't cause issues
	// when each handler gets its own copy

	// Create multiple handlers that will try to modify the event
	handlers := []evacuator.Handler{
		newMockHandler("handler1"),
		newMockHandler("handler2"),
		newMockHandler("handler3"),
	}

	// Create a test event
	originalEvent := evacuator.TerminationEvent{
		Hostname:   "test-host",
		PrivateIP:  "10.0.0.1",
		InstanceID: "i-123456",
		Reason:     evacuator.TerminationReasonSpot,
	}

	// Run multiple times to increase chance of detecting race conditions
	for i := 0; i < 100; i++ {
		ctx := context.Background()
		config := &evacuator.Config{
			NodeName: fmt.Sprintf("node-%d", i),
		}
		evacuator.SetGlobalConfig(config)

		var handlerWg sync.WaitGroup
		for _, handler := range handlers {
			handler := handler // Capture loop variable
			handlerWg.Add(1)
			go func() {
				defer handlerWg.Done()

				// Create a copy for this handler
				handlerEvent := originalEvent
				if config.NodeName != "" {
					handlerEvent.Hostname = config.NodeName
				}

				handlerCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()

				_ = handler.HandleTermination(handlerCtx, handlerEvent)
			}()
		}

		handlerWg.Wait()
	}

	// Verify original event wasn't modified
	if originalEvent.Hostname != "test-host" {
		t.Errorf("original event was modified: expected hostname 'test-host', got '%s'", originalEvent.Hostname)
	}

	evacuator.SetGlobalConfig(nil)
}

func TestHandlerResult(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		result HandlerResult
	}{
		{
			name: "successful result",
			result: HandlerResult{
				HandlerName: "test-handler",
				Error:       nil,
				ProcessedAt: now,
			},
		},
		{
			name: "failed result",
			result: HandlerResult{
				HandlerName: "test-handler",
				Error:       fmt.Errorf("test error"),
				ProcessedAt: now,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.result.HandlerName == "" {
				t.Error("expected handler name to be set")
			}
			if tt.result.ProcessedAt.IsZero() {
				t.Error("expected processed at to be set")
			}
		})
	}
}

func TestGracefulShutdownTimeout(t *testing.T) {
	// Verify the constant is set appropriately
	if GracefulShutdownTimeout < 5*time.Second {
		t.Error("GracefulShutdownTimeout is too short")
	}
	if GracefulShutdownTimeout > 30*time.Second {
		t.Error("GracefulShutdownTimeout is too long")
	}
}

func TestSetupLogger(t *testing.T) {
	tests := []struct {
		name    string
		config  evacuator.LogConfig
		wantErr bool
	}{
		{
			name: "valid json logger",
			config: evacuator.LogConfig{
				Level:  "info",
				Format: "json",
			},
			wantErr: false,
		},
		{
			name: "valid text logger",
			config: evacuator.LogConfig{
				Level:  "debug",
				Format: "text",
			},
			wantErr: false,
		},
		{
			name: "invalid log level",
			config: evacuator.LogConfig{
				Level:  "invalid",
				Format: "json",
			},
			wantErr: true,
		},
		{
			name: "invalid log format",
			config: evacuator.LogConfig{
				Level:  "info",
				Format: "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evacuator.SetGlobalConfig(&evacuator.Config{
				Log: tt.config,
			})

			logger, err := setupLogger()
			if (err != nil) != tt.wantErr {
				t.Errorf("setupLogger() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && logger == nil {
				t.Error("expected logger to be non-nil")
			}

			evacuator.SetGlobalConfig(nil)
		})
	}
}
