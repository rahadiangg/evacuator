package evacuator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// GcpProvider is an implementation of the Provider interface for GCP.
type GcpProvider struct {
	httpClient *http.Client
	logger     *slog.Logger
	mu         sync.Mutex // protects against overlapping spot checks
}

const (
	GcpMetaDataBaseUrl = "http://metadata.google.internal/computeMetadata/v1/instance"

	// gcp metadata endpoint
	GcpMetaDataSpotUrl       = GcpMetaDataBaseUrl + "/meta-data/spot/instance-action"
	GcpMetaDataHostnameUrl   = GcpMetaDataBaseUrl + "/hostname"
	GcpMetaDataInstanceIdUrl = GcpMetaDataBaseUrl + "/id"
	GcpMetaDataLocalIpUrl    = GcpMetaDataBaseUrl + "/network-interfaces/0/ip"
)

func NewGcpProvider(client *http.Client, logger *slog.Logger) *GcpProvider {
	return &GcpProvider{
		httpClient: client,
		logger:     logger,
	}
}

func (p *GcpProvider) Name() ProviderName {
	return ProviderGcp
}

func (p *GcpProvider) IsSupported(ctx context.Context) bool {

	_, err := p.doMetadataRequest(ctx, GcpMetaDataHostnameUrl)
	if err != nil {
		p.logger.Debug("fail to detect a gcp provider", "error", err.Error(), "provider_name", p.Name())
		return false
	}

	p.logger.Info("gcp provider detected", "provider_name", p.Name())
	return true
}

func (p *GcpProvider) StartMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	go p.startMonitoring(ctx, e)
	p.logger.Info("gcp provider monitoring started", "provider_name", p.Name())
}

func (p *GcpProvider) startMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	config := GetProviderConfig()
	interval, err := time.ParseDuration(config.PollInterval)
	if err != nil {
		p.logger.Warn("failed to parse poll interval", "error", err.Error(), "provider_name", p.Name())
		p.logger.Warn("using default interval of 3 seconds", "provider_name", p.Name())
		interval = 3 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Use mutex to prevent overlapping executions
			if !p.mu.TryLock() {
				p.logger.Debug("spot termination check already in progress, skipping", "provider_name", p.Name())
				continue
			}

			// Check for spot termination
			terminationDetected, err := p.isSpotTerminationDetected(ctx)
			if err != nil {
				p.logger.Error("failed to detect spot termination", "error", err.Error(), "provider_name", p.Name())
				p.mu.Unlock()
				continue
			}

			if terminationDetected {
				p.logger.Info("spot termination detected", "provider_name", p.Name())

				// Handle termination in a separate goroutine but keep the mutex locked
				// to prevent further ticker executions
				go func() {
					defer p.mu.Unlock()
					p.logger.Info("monitoring will be stopped and continue to handler", "provider_name", p.Name())

					t := p.getInstanceMetadatas(ctx)
					e <- t
				}()

				// Stop the ticker and exit the monitoring loop
				return
			}

			p.mu.Unlock()

		case <-ctx.Done():
			return
		}
	}
}

func (p *GcpProvider) isSpotTerminationDetected(ctx context.Context) (bool, error) {

	// Get spot instance action metadata
	spotInfo, err := p.doMetadataRequest(ctx, GcpMetaDataSpotUrl)
	if err != nil {
		return false, err
	}

	if spotInfo == "FALSE" {
		p.logger.Debug("no spot termination detected", "provider_name", p.Name())
	}

	return true, nil
}

func (p *GcpProvider) getInstanceMetadatas(ctx context.Context) TerminationEvent {
	var t TerminationEvent

	// Get hostname - log error but continue
	if hostname, err := p.doMetadataRequest(ctx, GcpMetaDataHostnameUrl); err != nil {
		p.logger.Error("failed to get hostname", "error", err.Error(), "provider_name", p.Name())
		t.Hostname = "unknown"
	} else {
		t.Hostname = hostname
	}

	// Get private IP - log error but continue
	if privateIP, err := p.doMetadataRequest(ctx, GcpMetaDataLocalIpUrl); err != nil {
		p.logger.Error("failed to get private IP", "error", err.Error(), "provider_name", p.Name())
		t.PrivateIP = "unknown"
	} else {
		t.PrivateIP = privateIP
	}

	// Get instance ID - log error but continue
	if instanceID, err := p.doMetadataRequest(ctx, GcpMetaDataInstanceIdUrl); err != nil {
		p.logger.Error("failed to get instance ID", "error", err.Error(), "provider_name", p.Name())
		t.InstanceID = "unknown"
	} else {
		t.InstanceID = instanceID
	}

	t.Reason = TerminationReasonSpot

	return t
}

func (p *GcpProvider) doMetadataRequest(ctx context.Context, url string) (string, error) {

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("got %d as http request", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	bodyStr := string(body)

	return bodyStr, nil
}
