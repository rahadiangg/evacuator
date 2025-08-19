package evacuator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// AwsProvider is an implementation of the Provider interface for AWS.
type AwsProvider struct {
	httpClient *http.Client
	logger     *slog.Logger
	mu         sync.Mutex // protects against overlapping spot checks
}

const (
	AwsMetaDataBaseUrl = "http://169.254.169.254/latest"

	// token endpoint
	AwsMetaDataTokenUrl = AwsMetaDataBaseUrl + "/api/token"

	// aws metadata endpoint
	AwsMetaDataSpotUrl       = AwsMetaDataBaseUrl + "/meta-data/spot/instance-action"
	AwsMetaDataHostnameUrl   = AwsMetaDataBaseUrl + "/meta-data/hostname"
	AwsMetaDataInstanceIdUrl = AwsMetaDataBaseUrl + "/meta-data/instance-id"
	AwsMetaDataLocalIpUrl    = AwsMetaDataBaseUrl + "/meta-data/local-ipv4"
)

type AwsResponseSpot struct {
	Action string    `json:"action"`
	Time   time.Time `json:"time"`
}

func NewAwsProvider(client *http.Client, logger *slog.Logger) *AwsProvider {
	return &AwsProvider{
		httpClient: client,
		logger:     logger,
	}
}

func (p *AwsProvider) Name() ProviderName {
	return ProviderAWS
}

func (p *AwsProvider) IsSupported(ctx context.Context) bool {

	_, err := p.doMetadataRequest(ctx, AwsMetaDataHostnameUrl)
	if err != nil {
		p.logger.Debug("fail to detect aws provider", "error", err.Error())
		return false
	}

	p.logger.Info("aws provider detected")
	return true
}

func (p *AwsProvider) StartMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	go p.startMonitoring(ctx, e)
	p.logger.Info("aws provider monitoring started")
}

func (p *AwsProvider) startMonitoring(ctx context.Context, e chan<- TerminationEvent) {

	config := GetProviderConfig()
	interval, err := time.ParseDuration(config.PollInterval)
	if err != nil {
		p.logger.Warn("failed to parse poll interval", "error", err.Error())
		p.logger.Warn("using default interval of 3 seconds")
		interval = 3 * time.Second
	}

	ticker := time.NewTicker(interval)
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

func (p *AwsProvider) isSpotTerminationDetected(ctx context.Context) (bool, error) {

	// Get spot instance action metadata
	spotInfo, err := p.doMetadataRequest(ctx, AwsMetaDataSpotUrl)
	if err != nil {
		return false, err
	}

	var r AwsResponseSpot

	if err := json.Unmarshal([]byte(spotInfo), &r); err != nil {
		return false, fmt.Errorf("failed to unmarshal spot instance action: %w", err)
	}

	if r.Action != "stop" && r.Action != "terminate" {
		return false, fmt.Errorf("unexpected spot instance action: %s", r.Action)
	}

	return true, nil
}

func (p *AwsProvider) getInstanceMetadatas(ctx context.Context) TerminationEvent {
	var t TerminationEvent

	// Get hostname - log error but continue
	if hostname, err := p.doMetadataRequest(ctx, AwsMetaDataHostnameUrl); err != nil {
		p.logger.Error("failed to get hostname", "error", err.Error())
		t.Hostname = "unknown"
	} else {
		t.Hostname = hostname
	}

	// Get private IP - log error but continue
	if privateIP, err := p.doMetadataRequest(ctx, AwsMetaDataLocalIpUrl); err != nil {
		p.logger.Error("failed to get private IP", "error", err.Error())
		t.PrivateIP = "unknown"
	} else {
		t.PrivateIP = privateIP
	}

	// Get instance ID - log error but continue
	if instanceID, err := p.doMetadataRequest(ctx, AwsMetaDataInstanceIdUrl); err != nil {
		p.logger.Error("failed to get instance ID", "error", err.Error())
		t.InstanceID = "unknown"
	} else {
		t.InstanceID = instanceID
	}

	t.Reason = TerminationReasonSpot

	return t
}

func (p *AwsProvider) getMetadataToken(ctx context.Context) (string, error) {
	// Get token for authentication next request
	req, err := http.NewRequestWithContext(ctx, "PUT", AwsMetaDataTokenUrl, nil)
	if err != nil {
		return "", err
	}

	// set header
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "60")

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

func (p *AwsProvider) doMetadataRequest(ctx context.Context, url string) (string, error) {
	token, err := p.getMetadataToken(ctx)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-aws-ec2-metadata-token", token)

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
