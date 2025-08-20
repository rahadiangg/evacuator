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

// TencentProvider is an implementation of the Provider interface for Tencent.
type TencentProvider struct {
	httpClient *http.Client
	logger     *slog.Logger
	mu         sync.Mutex // protects against overlapping spot checks
}

const (
	TencentMetaDataBaseUrl = "http://metadata.tencentyun.com/latest"

	// tencent metadata endpoint
	TencentMetaDataSpotUrl       = TencentMetaDataBaseUrl + "/meta-data/instance/spot/termination-time"
	TencentMetaDataHostnameUrl   = TencentMetaDataBaseUrl + "/meta-data/hostname"
	TencentMetaDataInstanceIdUrl = TencentMetaDataBaseUrl + "/meta-data/instance-id"
	TencentMetaDataLocalIpUrl    = TencentMetaDataBaseUrl + "/meta-data/local-ipv4"
)

func NewTencentProvider(client *http.Client, logger *slog.Logger) *TencentProvider {
	return &TencentProvider{
		httpClient: client,
		logger:     logger,
	}
}

func (p *TencentProvider) Name() ProviderName {
	return ProviderTencent
}

func (p *TencentProvider) IsSupported(ctx context.Context) bool {

	_, err := p.doMetadataRequest(ctx, TencentMetaDataHostnameUrl)
	if err != nil {
		p.logger.Debug("fail to detect tencent provider", "error", err.Error(), "provider", p.Name())
		return false
	}

	p.logger.Info("tencent provider detected", "provider", p.Name())
	return true
}

func (p *TencentProvider) StartMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	go p.startMonitoring(ctx, e)
	p.logger.Info("tencent provider monitoring started", "provider", p.Name())
}

func (p *TencentProvider) startMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	config := GetProviderConfig()
	interval, err := time.ParseDuration(config.PollInterval)
	if err != nil {
		p.logger.Warn("failed to parse poll interval", "error", err.Error(), "provider", p.Name())
		p.logger.Warn("using default interval of 3 seconds", "provider", p.Name())
		interval = 3 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Use mutex to prevent overlapping executions
			if !p.mu.TryLock() {
				p.logger.Debug("spot termination check already in progress, skipping", "provider", p.Name())
				continue
			}

			// Check for spot termination
			terminationDetected, err := p.isSpotTerminationDetected(ctx)
			if err != nil {
				p.logger.Error("failed to detect spot termination", "error", err.Error(), "provider", p.Name())
				p.mu.Unlock()
				continue
			}

			if terminationDetected {
				p.logger.Info("spot termination detected", "provider", p.Name())

				// Handle termination in a separate goroutine but keep the mutex locked
				// to prevent further ticker executions
				go func() {
					defer p.mu.Unlock()
					p.logger.Info("monitoring will be stopped and continue to handler", "provider", p.Name())

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

func (p *TencentProvider) isSpotTerminationDetected(ctx context.Context) (bool, error) {

	// Get spot instance action metadata
	_, err := p.doMetadataRequest(ctx, TencentMetaDataSpotUrl)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (p *TencentProvider) getInstanceMetadatas(ctx context.Context) TerminationEvent {
	var t TerminationEvent

	// Get hostname - log error but continue
	if hostname, err := p.doMetadataRequest(ctx, TencentMetaDataHostnameUrl); err != nil {
		p.logger.Error("failed to get hostname", "error", err.Error(), "provider", p.Name())
		t.Hostname = "unknown"
	} else {
		t.Hostname = hostname
	}

	// Get private IP - log error but continue
	if privateIP, err := p.doMetadataRequest(ctx, TencentMetaDataLocalIpUrl); err != nil {
		p.logger.Error("failed to get private IP", "error", err.Error(), "provider", p.Name())
		t.PrivateIP = "unknown"
	} else {
		t.PrivateIP = privateIP
	}

	// Get instance ID - log error but continue
	if instanceID, err := p.doMetadataRequest(ctx, TencentMetaDataInstanceIdUrl); err != nil {
		p.logger.Error("failed to get instance ID", "error", err.Error(), "provider", p.Name())
		t.InstanceID = "unknown"
	} else {
		t.InstanceID = instanceID
	}

	t.Reason = TerminationReasonSpot

	return t
}

func (p *TencentProvider) doMetadataRequest(ctx context.Context, url string) (string, error) {

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

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
