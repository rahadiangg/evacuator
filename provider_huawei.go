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

// HuaweiProvider is an implementation of the Provider interface for Huawei.
type HuaweiProvider struct {
	httpClient *http.Client
	logger     *slog.Logger
	mu         sync.Mutex // protects against overlapping spot checks
}

const (
	HuaweiMetaDataBaseUrl = "http://169.254.169.254"

	// token endpoint
	HuaweiMetaDataTokenUrl = HuaweiMetaDataBaseUrl + "/meta-data/latest/api/token"

	// huawei metadata endpoint
	HuaweiMetaDataSpotUrl       = HuaweiMetaDataBaseUrl + "/openstack/latest/spot/instance-action"
	HuaweiMetaDataHostnameUrl   = HuaweiMetaDataBaseUrl + "/latest/meta-data/hostname"
	HuaweiMetaDataInstanceIdUrl = HuaweiMetaDataBaseUrl + "-"
	HuaweiMetaDataLocalIpUrl    = HuaweiMetaDataBaseUrl + "/latest/meta-data/local-ipv4"
)

func NewHuaweiProvider(client *http.Client, logger *slog.Logger) *HuaweiProvider {
	return &HuaweiProvider{
		httpClient: client,
		logger:     logger,
	}
}

func (p *HuaweiProvider) Name() ProviderName {
	return ProviderHuawei
}

func (p *HuaweiProvider) IsSupported(ctx context.Context) bool {

	_, err := p.doMetadataRequest(ctx, HuaweiMetaDataHostnameUrl)
	if err != nil {
		p.logger.Debug("fail to detect huawei provider", "error", err.Error(), "provider", p.Name())
		return false
	}

	p.logger.Info("huawei provider detected", "provider", p.Name())
	return true
}

func (p *HuaweiProvider) StartMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	go p.startMonitoring(ctx, e)
	p.logger.Info("huawei provider monitoring started", "provider", p.Name())
}

func (p *HuaweiProvider) startMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	config := GetProviderConfig()

	ticker := time.NewTicker(config.PollInterval)
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

func (p *HuaweiProvider) isSpotTerminationDetected(ctx context.Context) (bool, error) {

	// Get spot instance action metadata
	_, err := p.doMetadataRequest(ctx, HuaweiMetaDataSpotUrl)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (p *HuaweiProvider) getInstanceMetadatas(ctx context.Context) TerminationEvent {
	var t TerminationEvent

	// Get hostname - log error but continue
	if hostname, err := p.doMetadataRequest(ctx, HuaweiMetaDataHostnameUrl); err != nil {
		p.logger.Error("failed to get hostname", "error", err.Error(), "provider", p.Name())
		t.Hostname = "unknown"
	} else {
		t.Hostname = hostname
	}

	// Get private IP - log error but continue
	if privateIP, err := p.doMetadataRequest(ctx, HuaweiMetaDataLocalIpUrl); err != nil {
		p.logger.Error("failed to get private IP", "error", err.Error(), "provider", p.Name())
		t.PrivateIP = "unknown"
	} else {
		t.PrivateIP = privateIP
	}

	t.InstanceID = "unknown" // Huawei does not provide instance ID in metadata

	t.Reason = TerminationReasonSpot

	return t
}

func (p *HuaweiProvider) getMetadataToken(ctx context.Context) (string, error) {
	// Get token for authentication next request
	req, err := http.NewRequestWithContext(ctx, "PUT", HuaweiMetaDataTokenUrl, nil)
	if err != nil {
		return "", err
	}

	// set header
	req.Header.Set("X-Metadata-Token-Ttl-Seconds", "60")

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

func (p *HuaweiProvider) doMetadataRequest(ctx context.Context, url string) (string, error) {
	token, err := p.getMetadataToken(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Metadata-Token", token)

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
