package cloud

import (
	"context"
	"fmt"
	"sync"
)

// Registry implements CloudProviderRegistry
type Registry struct {
	providers map[string]CloudProvider
	mu        sync.RWMutex
}

// NewRegistry creates a new cloud provider registry
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]CloudProvider),
	}
}

// RegisterProvider registers a new cloud provider
func (r *Registry) RegisterProvider(provider CloudProvider) error {
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}

	name := provider.Name()
	if name == "" {
		return fmt.Errorf("provider name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s is already registered", name)
	}

	r.providers[name] = provider
	return nil
}

// GetProvider returns a provider by name
func (r *Registry) GetProvider(name string) (CloudProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}

	return provider, nil
}

// GetSupportedProviders returns all providers that are supported in the current environment
func (r *Registry) GetSupportedProviders(ctx context.Context) ([]CloudProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var supported []CloudProvider
	for _, provider := range r.providers {
		if provider.IsSupported(ctx) {
			supported = append(supported, provider)
		}
	}

	return supported, nil
}

// DetectCurrentProvider returns the first supported cloud provider in the current environment
// This is more appropriate for DaemonSet deployment where each pod handles one node
func (r *Registry) DetectCurrentProvider(ctx context.Context) (CloudProvider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if provider.IsSupported(ctx) {
			return provider, nil
		}
	}

	return nil, fmt.Errorf("no supported cloud provider detected")
}

// ListProviders returns all registered provider names
func (r *Registry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names
}
