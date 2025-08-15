package cloud

import (
	"context"
	"time"
)

// TerminationReason represents the reason for node termination
type TerminationReason string

const (
	// SpotInstanceTermination indicates the node is being terminated due to spot instance interruption
	SpotInstanceTermination TerminationReason = "spot-instance-termination"
	// MaintenanceEvent indicates the node is being terminated due to scheduled maintenance
	MaintenanceEvent TerminationReason = "maintenance-event"
	// InstanceRetirement indicates the node is being terminated due to instance retirement
	InstanceRetirement TerminationReason = "instance-retirement"
	// UserInitiated indicates the node termination was initiated by user
	UserInitiated TerminationReason = "user-initiated"
	// SystemShutdown indicates the node is being terminated due to system shutdown
	SystemShutdown TerminationReason = "system-shutdown"
	// Unknown indicates the termination reason is unknown or not specified
	Unknown TerminationReason = "unknown"
)

// TerminationEvent represents a node termination event
type TerminationEvent struct {
	// NodeID is the unique identifier of the node (instance ID, VM name, etc.)
	NodeID string `json:"node_id"`

	// NodeName is the Kubernetes node name
	NodeName string `json:"node_name"`

	// Reason indicates why the node is being terminated
	Reason TerminationReason `json:"reason"`

	// TerminationTime is when the node will be terminated
	TerminationTime time.Time `json:"termination_time"`

	// NoticeTime is when the termination notice was received
	NoticeTime time.Time `json:"notice_time"`

	// GracePeriod is the time between notice and actual termination
	GracePeriod time.Duration `json:"grace_period"`

	// CloudProvider indicates which cloud provider issued this event
	CloudProvider string `json:"cloud_provider"`

	// Region is the cloud region where the node is located
	Region string `json:"region"`

	// Zone is the availability zone where the node is located
	Zone string `json:"zone"`

	// InstanceType is the type/size of the instance
	InstanceType string `json:"instance_type"`

	// Metadata contains provider-specific additional information
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// CloudProvider represents a cloud provider interface for detecting termination events
type CloudProvider interface {
	// Name returns the name of the cloud provider (e.g., "aws", "gcp", "alibaba")
	Name() string

	// IsSupported checks if the current environment supports this cloud provider
	IsSupported(ctx context.Context) bool

	// StartMonitoring starts monitoring for termination events
	// Returns a channel that will receive termination events
	StartMonitoring(ctx context.Context) (<-chan TerminationEvent, error)

	// StopMonitoring stops the monitoring process
	StopMonitoring() error

	// GetNodeInfo retrieves current node information
	GetNodeInfo(ctx context.Context) (*NodeInfo, error)

	// ValidateConfiguration validates the provider configuration
	ValidateConfiguration() error
}

// NodeInfo represents information about the current node
type NodeInfo struct {
	// NodeID is the unique identifier of the node
	NodeID string `json:"node_id"`

	// NodeName is the Kubernetes node name
	NodeName string `json:"node_name"`

	// CloudProvider is the cloud provider name
	CloudProvider string `json:"cloud_provider"`

	// Region is the cloud region
	Region string `json:"region"`

	// Zone is the availability zone
	Zone string `json:"zone"`

	// InstanceType is the instance type/size
	InstanceType string `json:"instance_type"`

	// IsSpotInstance indicates if this is a spot/preemptible instance
	IsSpotInstance bool `json:"is_spot_instance"`

	// PublicIP is the public IP address of the node
	PublicIP string `json:"public_ip,omitempty"`

	// PrivateIP is the private IP address of the node
	PrivateIP string `json:"private_ip,omitempty"`

	// Labels contains node labels
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations contains node annotations
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ProviderConfig represents configuration for a cloud provider
type ProviderConfig struct {
	// Name is the provider name
	Name string `yaml:"name" json:"name"`

	// PollInterval is how often to check for termination events
	PollInterval time.Duration `yaml:"poll_interval" json:"poll_interval"`

	// Timeout is the timeout for API calls
	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	// Retries is the number of retries for failed requests
	Retries int `yaml:"retries" json:"retries"`
}

// EventHandler handles termination events
type EventHandler interface {
	// HandleTerminationEvent processes a termination event
	HandleTerminationEvent(ctx context.Context, event TerminationEvent) error

	// Name returns the handler name
	Name() string
}

// CloudProviderRegistry manages multiple cloud providers
type CloudProviderRegistry interface {
	// RegisterProvider registers a new cloud provider
	RegisterProvider(provider CloudProvider) error

	// GetProvider returns a provider by name
	GetProvider(name string) (CloudProvider, error)

	// DetectCurrentProvider returns the first supported cloud provider (for single-node DaemonSet deployment)
	DetectCurrentProvider(ctx context.Context) (CloudProvider, error)
}
