package http_server

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type AWSProxy struct {
	// TODO all these lookup providers, esp for the service, feel a bit strange
	// AWS Key id to secret
	keyLookupProvider LookupProvider[string, string]
	// incoming hostname to outgoing hostname
	hostLookupProvider LookupProvider[string, string]
	// incoming hostname to service provider
	serviceLookupProvider LookupProvider[string, AWSServiceProvider]
}

func NewAWSProxy(
	keyLookupProvider, hostLookupProvider LookupProvider[string, string],
	serviceLookupProvider LookupProvider[string, AWSServiceProvider],
) AWSProxy {
	panic("todo")
}

// Listen will listen on an interface:port pair
func (p *AWSProxy) Listen(addr string) error {
	panic("todo")
}

func (p *AWSProxy) handleRequest(w http.ResponseWriter, r *http.Request) error {
	parsedHeader := parseAuthHeader(r.Header.Get("Authorization"))

	ctx := context.Background() // todo fix this

	// Verify the request

	// Because we changed the host, we need to resign the request to the new host
	canonicalRequest := getCanonicalRequest(r)
	stringToSign := getStringToSign(r, canonicalRequest, parsedHeader.Credential.Region, parsedHeader.Credential.Service)

	// Look up key secret from ID
	keySecret, err := p.keyLookupProvider.Lookup(ctx, parsedHeader.Credential.KeyID)
	if err != nil {
		// TODO respond
		return fmt.Errorf("error looking up key: %w", err)
	}

	signingKey := getSigningKey(r, keySecret, parsedHeader.Credential.Region, parsedHeader.Credential.Service)
	signature := fmt.Sprintf("%x", getHMAC(signingKey, []byte(stringToSign)))

	if signature != parsedHeader.Signature {
		// TODO respond
		return ErrInvalidSignature
	}

	proxiedRequest := ProxiedRequest{
		Request:           r,
		OriginalHost:      r.Host,
		Region:            parsedHeader.Credential.Region,
		KeyID:             parsedHeader.Credential.KeyID,
		Service:           parsedHeader.Credential.Service,
		XAMZDate:          parsedHeader.Credential.Date,
		keyLookupProvider: p.keyLookupProvider,
		responseWriter:    w,
	}

	serviceProvider, err := p.serviceLookupProvider.Lookup(ctx, r.Host)
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
