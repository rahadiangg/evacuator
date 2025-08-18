package evacuator

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
	HandleTermination(e <-chan TerminationEvent)
}
