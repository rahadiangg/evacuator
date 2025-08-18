package evacuator

import "context"

type TerminationEvent struct {
	Hostname   string
	PrivateIP  string
	InstanceID string
	Reason     TerminationReason
}

type TerminationReason string

const (
	TerminationReasonSpot        TerminationReason = "spot termination"
	TerminationReasonMaintenance TerminationReason = "maintenance termination"
)

type Handler interface {
	// Process a single termination event and return any error
	HandleTermination(ctx context.Context, event TerminationEvent) error

	// Get handler name for logging and identification
	Name() string
}
