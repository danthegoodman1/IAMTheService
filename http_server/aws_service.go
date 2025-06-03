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
