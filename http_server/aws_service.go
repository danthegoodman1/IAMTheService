package http_server

import (
	"context"
	"net/http"
)

type AWSServiceProvider interface {
	// HandleRequest will handle a request for a given service.
	// Returning a *http.Response will stream that response to the client
	HandleRequest(ctx context.Context, request *ProxiedRequest) (*http.Response, error)
}
