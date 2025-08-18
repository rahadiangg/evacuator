package evacuator

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// DummyProvider is an implementation of the Provider interface for AWS.
type DummyProvider struct {
	httpClient    *http.Client
	logger        *slog.Logger
	DetectionWait time.Duration
}

func NewDummyProvider(client *http.Client, logger *slog.Logger, detectionWait time.Duration) *DummyProvider {
	return &DummyProvider{
		httpClient:    client,
		logger:        logger,
		DetectionWait: detectionWait,
	}
}

func (p *DummyProvider) Name() ProviderName {
	return ProviderDummy
}

func (p *DummyProvider) IsSupported() bool {
	p.logger.Info("dummy provider detected")
	return true
}

func (p *DummyProvider) StartMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	go func() {
		time.Sleep(p.DetectionWait)
		p.logger.Info("spot termination detected")
		p.logger.Info("monitoring will be stopped and continue to handler")

		t := TerminationEvent{
			Hostname:   "dummy",
			PrivateIP:  "172.16.1.1",
			InstanceID: "dummy-instance-id",
			Reason:     TerminationReasonSpot,
		}
		e <- t
	}()
	p.logger.Info("dummy provider monitoring started")
}
