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

func (p *AwsProvider) IsSupported() bool {

	_, err := p.doMetadataRequest(AwsMetaDataHostnameUrl)
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

	ticker := time.NewTicker(2 * time.Second)
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
			err, terminationDetected := p.isSpotTerminationDetected()
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

					t := p.getInstanceMetadatas()
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

func (p *AwsProvider) isSpotTerminationDetected() (error, bool) {
	// Get spot instance action metadata
	spotInfo, err := p.doMetadataRequest(AwsMetaDataSpotUrl)
	if err != nil {
		return err, false
	}

	var r AwsResponseSpot

	if err := json.Unmarshal([]byte(spotInfo), &r); err != nil {
		return fmt.Errorf("failed to unmarshal spot instance action: %w", err), false
	}

	if r.Action != "stop" && r.Action != "terminate" {
		return fmt.Errorf("unexpected spot instance action: %s", r.Action), false
	}

	return nil, true
}

func (p *AwsProvider) getInstanceMetadatas() TerminationEvent {
	var t TerminationEvent
	t.Hostname, _ = p.doMetadataRequest(AwsMetaDataHostnameUrl)
	t.PrivateIP, _ = p.doMetadataRequest(AwsMetaDataLocalIpUrl)
	t.InstanceID, _ = p.doMetadataRequest(AwsMetaDataInstanceIdUrl)

	return t
}

func (p *AwsProvider) getMetadataToken() (string, error) {

	// Get token for authentication next request
	req, err := http.NewRequest("PUT", AwsMetaDataTokenUrl, nil)
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

func (p *AwsProvider) doMetadataRequest(url string) (string, error) {

	token, err := p.getMetadataToken()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", url, nil)
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
