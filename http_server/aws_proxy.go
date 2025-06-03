package http_server

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type LookupFunc[TKey any, TVal any] func(ctx context.Context, key TKey) (TVal, error)

type AWSProxy struct {
	// AWS Key id to secret
	KeyLookupFunc LookupFunc[string, string]
	// incoming hostname to outgoing hostname
	HostLookupFunc LookupFunc[string, string]
	// incoming hostname to service provider
	ServiceLookupFunc LookupFunc[string, AWSServiceProvider]
}

func (p *AWSProxy) handleRequest(w http.ResponseWriter, r *http.Request) error {
	ctx := context.Background()

	parsedHeader := parseAuthHeader(r.Header.Get("Authorization"))

	// Look up key secret from ID
	keySecret, err := p.KeyLookupFunc(ctx, parsedHeader.Credential.KeyID)
	if err != nil {
		// TODO respond
		return fmt.Errorf("error looking up key: %w", err)
	}

	signature := generateSigV4(r, parsedHeader, keySecret)

	if signature != parsedHeader.Signature {
		// TODO respond
		return fmt.Errorf("invalid signature")
	}

	proxiedRequest := ProxiedRequest{
		Request:        r,
		OriginalHost:   r.Host,
		Region:         parsedHeader.Credential.Region,
		KeyID:          parsedHeader.Credential.KeyID,
		Service:        parsedHeader.Credential.Service,
		XAMZDate:       parsedHeader.Credential.Date,
		responseWriter: w,
		parsedHeader:   parsedHeader,
	}

	serviceProvider, err := p.ServiceLookupFunc(ctx, r.Host)
	if err != nil {
		// TODO respond
		return fmt.Errorf("error looking up service provider for host %s: %w", r.Host, err)
	}

	res, err := serviceProvider.HandleRequest(ctx, &proxiedRequest)
	if err != nil {
		// TODO respond
		return fmt.Errorf("error handling request: %w", err)
	}

	if proxiedRequest.hijacked {
		// We are no longer responsible for this request
		return nil
	}

	w.WriteHeader(res.StatusCode)
	// Write the headers
	for key, vals := range res.Header {
		for _, val := range vals {
			w.Header().Add(key, val)
		}
	}

	// Stream the response
	defer res.Body.Close()
	if _, err = io.Copy(w, res.Body); err != nil {
		return fmt.Errorf("error in io.Copy of response body: %w", err)
	}

	return nil
}
