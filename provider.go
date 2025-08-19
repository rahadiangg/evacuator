package evacuator

import "context"

type Provider interface {
	// Get the provider name
	Name() ProviderName

	// Detect current provider
	IsSupported(ctx context.Context) bool

	// StartMonitoring
	StartMonitoring(ctx context.Context, e chan<- TerminationEvent)
}

type ProviderName string

const (
	ProviderDummy    ProviderName = "dummy"
	ProviderAWS      ProviderName = "aws"
	ProviderAlicloud ProviderName = "alicloud"
	ProviderGcp      ProviderName = "gcp"
	ProviderTencent  ProviderName = "tencent"
)
