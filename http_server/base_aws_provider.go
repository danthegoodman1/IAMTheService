package http_server

import (
	"context"
	"net/http"
	"strings"
)

// BaseAWSProvider provides common functionality for all AWS service providers
type BaseAWSProvider struct {
	serviceName string
}

// NewBaseAWSProvider creates a new base provider for the specified service
func NewBaseAWSProvider(serviceName string) *BaseAWSProvider {
	return &BaseAWSProvider{
		serviceName: serviceName,
	}
}

// ServiceName returns the AWS service name this provider handles
func (p *BaseAWSProvider) ServiceName() string {
	return p.serviceName
}

// CanHandleRequest determines if this provider can handle the given request
// based on the service name in the host (e.g., s3.amazonaws.com)
func (p *BaseAWSProvider) CanHandleRequest(request *ProxiedRequest) bool {
	host := request.Request.Host
	if host == "" {
		host = request.Request.URL.Host
	}

	// Check if the host contains the service name
	// This is a simple check and might need to be more sophisticated
	return strings.Contains(host, p.serviceName+".")
}

// HandleRequest implements the default proxying behavior
// This will be called when a specific service provider doesn't override a method
func (p *BaseAWSProvider) HandleRequest(ctx context.Context, request *ProxiedRequest) (*http.Response, error) {
	// Default behavior: proxy the request to the origin service
	// Determine the target host based on the service name
	targetHost := p.serviceName + ".amazonaws.com"
	return request.DoProxiedRequest(ctx, targetHost)
}

// ExtractOperationName attempts to determine the AWS API operation from the request
// This is a helper method that specific providers can use or override
func (p *BaseAWSProvider) ExtractOperationName(request *ProxiedRequest) string {
	// This is a simplified implementation and may need to be enhanced
	// for specific services

	path := request.Request.URL.Path
	method := request.Request.Method

	// Extract operation based on HTTP method and path pattern
	// This is very service-specific and would need customization

	// Example for common patterns:
	switch method {
	case "GET":
		if path == "/" || path == "" {
			return "ListBuckets" // S3 example
		}
		if strings.HasSuffix(path, "/") {
			return "ListObjects" // S3 example
		}
		return "GetObject" // S3 example
	case "PUT":
		if strings.HasSuffix(path, "/") {
			return "CreateBucket" // S3 example
		}
		return "PutObject" // S3 example
	case "DELETE":
		return "DeleteObject" // S3 example
	}

	return "Unknown"
}
