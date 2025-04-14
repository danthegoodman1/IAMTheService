package http_server

import (
	"context"
	"net/http"
)

type AWSServiceProvider interface {
	// ServiceName returns the AWS service name this provider handles (e.g., "s3", "dynamodb")
	ServiceName() string

	// CanHandleRequest determines if this provider can handle the given request
	// based on the service and operation
	CanHandleRequest(request *ProxiedRequest) bool

	// HandleRequest will handle a request for a given service.
	// Returning a *http.Response will stream that response to the client
	HandleRequest(ctx context.Context, request *ProxiedRequest) (*http.Response, error)
}

// AWSServiceRegistry manages the registration and lookup of service providers
type AWSServiceRegistry struct {
	providers map[string]AWSServiceProvider
}

// NewAWSServiceRegistry creates a new registry for AWS service providers
func NewAWSServiceRegistry() *AWSServiceRegistry {
	return &AWSServiceRegistry{
		providers: make(map[string]AWSServiceProvider),
	}
}

// RegisterProvider adds a service provider to the registry
func (r *AWSServiceRegistry) RegisterProvider(provider AWSServiceProvider) {
	r.providers[provider.ServiceName()] = provider
}

// GetProviderForRequest finds the appropriate provider for a request
func (r *AWSServiceRegistry) GetProviderForRequest(request *ProxiedRequest) (AWSServiceProvider, bool) {
	for _, provider := range r.providers {
		if provider.CanHandleRequest(request) {
			return provider, true
		}
	}
	return nil, false
}
