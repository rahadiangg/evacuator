package alibaba

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rahadiangg/evacuator/pkg/cloud"
)

// HTTPError represents an HTTP error with status code
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

const ProviderAlibaba = "alibaba"

// Provider implements the CloudProvider interface for Alibaba Cloud
type Provider struct {
	config     *cloud.ProviderConfig
	httpClient *http.Client
	stopChan   chan struct{}
}

// Alibaba Cloud Instance Metadata Service endpoints
const (
	// Base metadata endpoint
	MetadataServiceBase = "http://100.100.100.200/latest"

	// Token endpoint for authentication
	TokenEndpoint = MetadataServiceBase + "/api/token"

	// Instance info endpoints
	InstanceIdentityEndpoint = MetadataServiceBase + "/meta-data/instance-identity/document"
	InstanceIDEndpoint       = MetadataServiceBase + "/meta-data/instance-id"
	InstanceTypeEndpoint     = MetadataServiceBase + "/meta-data/instance/instance-type"
	RegionEndpoint           = MetadataServiceBase + "/meta-data/region-id"
	ZoneEndpoint             = MetadataServiceBase + "/meta-data/zone-id"

	// Spot instance endpoints
	// According to Alibaba Cloud documentation:
	// - instance/spot/termination-time: Returns 404 if instance is available,
	//   or UTC timestamp (e.g., 2015-01-05T18:02:00Z) if instance is due to be reclaimed
	SpotTerminationEndpoint = MetadataServiceBase + "/meta-data/instance/spot/termination-time"
	SpotActionEndpoint      = MetadataServiceBase + "/meta-data/instance/spot/action"

	// Network endpoints
	PrivateIPEndpoint = MetadataServiceBase + "/meta-data/private-ipv4"
	PublicIPEndpoint  = MetadataServiceBase + "/meta-data/eipv4"
)

// NewProvider creates a new Alibaba Cloud provider
func NewProvider(config *cloud.ProviderConfig) *Provider {
	// Create a copy of the config to avoid mutating the input
	providerConfig := *config
	providerConfig.Name = ProviderAlibaba

	return &Provider{
		config: &providerConfig,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Name returns the provider name
func (p *Provider) Name() string {
	return ProviderAlibaba
}

// IsSupported checks if we're running on Alibaba Cloud by trying to access the metadata service
func (p *Provider) IsSupported(ctx context.Context) bool {
	// Try to get metadata token
	token, err := p.getMetadataToken(ctx)
	if err != nil {
		return false
	}

	// Try to access instance ID with token
	_, err = p.makeMetadataRequest(ctx, InstanceIDEndpoint, token)
	return err == nil
}

// StartMonitoring starts monitoring for Alibaba Cloud termination events
func (p *Provider) StartMonitoring(ctx context.Context) (<-chan cloud.TerminationEvent, error) {
	if err := p.ValidateConfiguration(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Use configurable buffer size, defaulting to 10 if not specified
	bufferSize := p.config.EventBufferSize
	if bufferSize <= 0 {
		bufferSize = 10
	}
	eventChan := make(chan cloud.TerminationEvent, bufferSize)
	p.stopChan = make(chan struct{})

	// Start monitoring goroutine
	go p.monitor(ctx, eventChan)

	return eventChan, nil
}

// StopMonitoring stops the monitoring process
func (p *Provider) StopMonitoring() error {
	if p.stopChan != nil {
		close(p.stopChan)
		p.stopChan = nil
	}
	return nil
}

// GetNodeInfo retrieves current Alibaba Cloud node information
func (p *Provider) GetNodeInfo(ctx context.Context) (*cloud.NodeInfo, error) {
	// Get metadata token
	token, err := p.getMetadataToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata token: %w", err)
	}

	// Get instance ID
	instanceID, err := p.makeMetadataRequest(ctx, InstanceIDEndpoint, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance ID: %w", err)
	}

	// Get instance type
	instanceType, err := p.makeMetadataRequest(ctx, InstanceTypeEndpoint, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance type: %w", err)
	}

	// Get region
	region, err := p.makeMetadataRequest(ctx, RegionEndpoint, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get region: %w", err)
	}

	// Get zone
	zone, err := p.makeMetadataRequest(ctx, ZoneEndpoint, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get zone: %w", err)
	}

	// Get private IP
	privateIP, err := p.makeMetadataRequest(ctx, PrivateIPEndpoint, token)
	if err != nil {
		privateIP = []byte("")
	}

	// Get public IP (may not exist)
	publicIP, err := p.makeMetadataRequest(ctx, PublicIPEndpoint, token)
	if err != nil {
		publicIP = []byte("")
	}

	// Check if this is a spot instance
	isSpot := p.isSpotInstance(ctx, token)

	return &cloud.NodeInfo{
		NodeID:         string(instanceID),
		NodeName:       string(instanceID), // Use instance ID as fallback when NODE_NAME not set
		CloudProvider:  ProviderAlibaba,
		Region:         string(region),
		Zone:           string(zone),
		InstanceType:   string(instanceType),
		IsSpotInstance: isSpot,
		PrivateIP:      string(privateIP),
		PublicIP:       string(publicIP),
		Labels: map[string]string{
			"alibabacloud.com/instance-type": string(instanceType),
		},
		Annotations: map[string]string{
			"alibabacloud.com/instance-id": string(instanceID),
		},
	}, nil
}

// ValidateConfiguration validates the Alibaba Cloud provider configuration
func (p *Provider) ValidateConfiguration() error {
	if p.config == nil {
		return fmt.Errorf("configuration is required")
	}

	if p.config.PollInterval < time.Second {
		return fmt.Errorf("poll interval must be at least 1 second")
	}

	if p.config.Timeout < time.Second {
		return fmt.Errorf("timeout must be at least 1 second")
	}

	if p.config.EventBufferSize < 0 {
		return fmt.Errorf("event buffer size cannot be negative")
	}

	return nil
}

// monitor continuously checks for termination events
func (p *Provider) monitor(ctx context.Context, eventChan chan<- cloud.TerminationEvent) {
	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()
	defer close(eventChan)

	for {
		select {
		case <-ctx.Done():
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			if event := p.checkForTermination(ctx); event != nil {
				select {
				case eventChan <- *event:
				case <-ctx.Done():
					return
				case <-p.stopChan:
					return
				}
			}
		}
	}
}

// checkForTermination checks for spot instance termination or maintenance events
func (p *Provider) checkForTermination(ctx context.Context) *cloud.TerminationEvent {
	// Get metadata token
	token, err := p.getMetadataToken(ctx)
	if err != nil {
		return nil
	}

	// Check for spot instance termination
	if event := p.checkSpotTermination(ctx, token); event != nil {
		return event
	}

	return nil
}

// checkSpotTermination checks for spot instance termination notice
func (p *Provider) checkSpotTermination(ctx context.Context, token string) *cloud.TerminationEvent {
	// Check termination time following official Alibaba Cloud documentation:
	// - If 404 is returned, the instance is available
	// - If a UTC timestamp is returned, the instance is due to be reclaimed at that time
	terminationTimeResp, err := p.makeMetadataRequest(ctx, SpotTerminationEndpoint, token)
	if err != nil {
		// Check if it's a 404 error (instance is available)
		if httpErr, ok := err.(*HTTPError); ok && httpErr.StatusCode == http.StatusNotFound {
			return nil // Instance is available, no termination notice
		}
		return nil // Other errors, assume no termination notice
	}

	terminationTimeStr := string(terminationTimeResp)
	if terminationTimeStr == "" {
		return nil
	}

	// Parse termination time (Alibaba Cloud format: 2015-01-05T18:02:00Z)
	terminationTime, err := time.Parse(time.RFC3339, terminationTimeStr)
	if err != nil {
		return nil
	}

	// Get spot action if available
	actionResp, _ := p.makeMetadataRequest(ctx, SpotActionEndpoint, token)
	action := string(actionResp)

	nodeInfo, _ := p.GetNodeInfo(ctx)

	return &cloud.TerminationEvent{
		NodeID:          nodeInfo.NodeID,
		NodeName:        nodeInfo.NodeName,
		Reason:          cloud.SpotInstanceTermination,
		TerminationTime: terminationTime,
		NoticeTime:      time.Now(),
		GracePeriod:     time.Until(terminationTime),
		CloudProvider:   ProviderAlibaba,
		Region:          nodeInfo.Region,
		Zone:            nodeInfo.Zone,
		InstanceType:    nodeInfo.InstanceType,
		Metadata: map[string]interface{}{
			"spot_action":      action,
			"termination_time": terminationTimeStr,
		},
	}
}

// getMetadataToken gets an access token for Alibaba Cloud metadata service
// Following the official documentation example:
// TOKEN=`curl -X PUT "http://100.100.100.200/latest/api/token" -H "X-aliyun-ecs-metadata-token-ttl-seconds:<Validity period>"`
func (p *Provider) getMetadataToken(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "PUT", TokenEndpoint, nil)
	if err != nil {
		return "", err
	}

	// Set the token TTL header (validity period in seconds)
	req.Header.Set("X-aliyun-ecs-metadata-token-ttl-seconds", "60")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to get token: status %d", resp.StatusCode)
	}

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// makeMetadataRequest makes a request to the Alibaba Cloud metadata service with token authentication
// Following the official documentation pattern:
// curl -H "X-aliyun-ecs-metadata-token: $TOKEN" http://100.100.100.200/latest/meta-data/instance/spot/termination-time
func (p *Provider) makeMetadataRequest(ctx context.Context, endpoint, token string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add the authentication token header as shown in official documentation
	req.Header.Set("X-aliyun-ecs-metadata-token", token)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    fmt.Sprintf("metadata request failed for %s", endpoint),
		}
	}

	return io.ReadAll(resp.Body)
}

// isSpotInstance checks if this is a spot instance
func (p *Provider) isSpotInstance(ctx context.Context, token string) bool {
	// According to Alibaba Cloud documentation:
	// Try to access spot termination endpoint - if it returns 404, it's available (but is a spot instance)
	// If it returns other errors or success, it's also a spot instance
	// If the endpoint doesn't exist at all, it's likely not a spot instance
	_, err := p.makeMetadataRequest(ctx, SpotTerminationEndpoint, token)
	if err != nil {
		// Check if it's a 404 error (instance is available spot instance)
		if httpErr, ok := err.(*HTTPError); ok && httpErr.StatusCode == http.StatusNotFound {
			return true // This is a spot instance that's currently available
		}
		return false // Other errors likely mean it's not a spot instance
	}
	return true // Successfully accessed spot endpoint, definitely a spot instance
}
