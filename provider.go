package evacuator

import "context"

type Provider interface {
	// Get the provider name
	Name() ProviderName

	// Detect current provider
	IsSupported() bool

	// StartMonitoring
	StartMonitoring(ctx context.Context)
}

type ProviderName string

const (
	ProviderAWS      ProviderName = "aws"
	ProviderAlicloud ProviderName = "alicloud"
	ProviderGcp      ProviderName = "gcp"
	ProviderTencent  ProviderName = "tencent"
)
