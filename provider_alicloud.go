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

// AlicloudProvider is an implementation of the Provider interface for Alicloud.
type AlicloudProvider struct {
	httpClient *http.Client
	logger     *slog.Logger
	mu         sync.Mutex // protects against overlapping spot checks
}

const (
	AlicloudMetaDataBaseUrl = "http://100.100.100.200/latest"

	// token endpoint
	AlicloudMetaDataTokenUrl = AlicloudMetaDataBaseUrl + "/api/token"

	// alicloud metadata endpoint
	AlicloudMetaDataSpotUrl       = AlicloudMetaDataBaseUrl + "/meta-data/instance/spot/termination-time"
	AlicloudMetaDataHostnameUrl   = AlicloudMetaDataBaseUrl + "/meta-data/hostname"
	AlicloudMetaDataInstanceIdUrl = AlicloudMetaDataBaseUrl + "/meta-data/instance-id"
	AlicloudMetaDataLocalIpUrl    = AlicloudMetaDataBaseUrl + "/meta-data/private-ipv4"

	// Alicloud metadata service timeout constants
	AlicloudMonitoringInterval = 2 * time.Second
)

func NewAlicloudProvider(client *http.Client, logger *slog.Logger) *AlicloudProvider {
	return &AlicloudProvider{
		httpClient: client,
		logger:     logger,
	}
}

func (p *AlicloudProvider) Name() ProviderName {
	return ProviderAlicloud
}

func (p *AlicloudProvider) IsSupported(ctx context.Context) bool {

	_, err := p.doMetadataRequest(ctx, AlicloudMetaDataHostnameUrl)
	if err != nil {
		p.logger.Debug("fail to detect alicloud provider", "error", err.Error())
		return false
	}

	p.logger.Info("alicloud provider detected")
	return true
}

func (p *AlicloudProvider) StartMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	go p.startMonitoring(ctx, e)
	p.logger.Info("alicloud provider monitoring started")
}

func (p *AlicloudProvider) startMonitoring(ctx context.Context, e chan<- TerminationEvent) {
	ticker := time.NewTicker(AlicloudMonitoringInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Use mutex to prevent overlapping executions
			if !p.mu.TryLock() {
				p.logger.Debug("spot termination check already in progress, skipping")
				continue
			}

			// Check for spot termination
			terminationDetected, err := p.isSpotTerminationDetected(ctx)
			if err != nil {
				p.logger.Error("failed to detect spot termination", "error", err.Error())
				p.mu.Unlock()
				continue
			}

			if terminationDetected {
				p.logger.Info("spot termination detected")

				// Handle termination in a separate goroutine but keep the mutex locked
				// to prevent further ticker executions
				go func() {
					defer p.mu.Unlock()
					p.logger.Info("monitoring will be stopped and continue to handler")

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

func (p *AlicloudProvider) isSpotTerminationDetected(ctx context.Context) (bool, error) {

	// Get spot instance action metadata
	_, err := p.doMetadataRequest(ctx, AlicloudMetaDataSpotUrl)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (p *AlicloudProvider) getInstanceMetadatas(ctx context.Context) TerminationEvent {
	var t TerminationEvent

	// Get hostname - log error but continue
	if hostname, err := p.doMetadataRequest(ctx, AlicloudMetaDataHostnameUrl); err != nil {
		p.logger.Error("failed to get hostname", "error", err.Error())
		t.Hostname = "unknown"
	} else {
		t.Hostname = hostname
	}

	// Get private IP - log error but continue
	if privateIP, err := p.doMetadataRequest(ctx, AlicloudMetaDataLocalIpUrl); err != nil {
		p.logger.Error("failed to get private IP", "error", err.Error())
		t.PrivateIP = "unknown"
	} else {
		t.PrivateIP = privateIP
	}

	// Get instance ID - log error but continue
	if instanceID, err := p.doMetadataRequest(ctx, AlicloudMetaDataInstanceIdUrl); err != nil {
		p.logger.Error("failed to get instance ID", "error", err.Error())
		t.InstanceID = "unknown"
	} else {
		t.InstanceID = instanceID
	}

	t.Reason = TerminationReasonSpot

	return t
}

func (p *AlicloudProvider) getMetadataToken(ctx context.Context) (string, error) {
	// Get token for authentication next request
	req, err := http.NewRequestWithContext(ctx, "PUT", AlicloudMetaDataTokenUrl, nil)
	if err != nil {
		return "", err
	}

	// set header
	req.Header.Set("X-aliyun-ecs-metadata-token-ttl-seconds", "60")

	// Doing request for get token
	res, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	// parse token
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}

	token := string(body)

	return token, nil
}

func (p *AlicloudProvider) doMetadataRequest(ctx context.Context, url string) (string, error) {
	token, err := p.getMetadataToken(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-aliyun-ecs-metadata-token", token)

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
