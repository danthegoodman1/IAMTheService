package http_server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

type ProxiedRequest struct {
	Request      *http.Request
	OriginalHost string
	Region       string
	KeyID        string
	Service      string
	XAMZDate     string
}

func cloneBody(orig io.ReadCloser) (io.ReadCloser, io.ReadCloser) {
	// Create two pipes.
	pr1, pw1 := io.Pipe()
	pr2, pw2 := io.Pipe()

	// Create a MultiWriter that writes to both pipe writers.
	multiWriter := io.MultiWriter(pw1, pw2)

	// Start a goroutine that copies data from the original stream
	// to both pipe writers concurrently.
	go func() {
		// Ensure both writers are closed when done.
		defer pw1.Close()
		defer pw2.Close()

		// Copy data from the original stream to both pipes.
		if _, err := io.Copy(multiWriter, orig); err != nil {
			// Propagate the error to both pipes.
			pw1.CloseWithError(err)
			pw2.CloseWithError(err)
		}

		// Close the original stream so we don't leak
		orig.Close()
	}()

	return pr1, pr2
}

// GetClonedBody will get a clone of the original request body that can be read, without breaking
// the original *http.Request.Body
func (r *ProxiedRequest) GetClonedBody() io.Reader {
	cloned, original := cloneBody(r.Request.Body)
	r.Request.Body = original

	return cloned
}

// DoRequest will do the original request, replacing the specified host
func (r *ProxiedRequest) DoRequest(ctx context.Context, host string) (*http.Response, error) {
	originalURL := r.Request.URL

	// set the new host
	r.Request.Host = host
	originalURL.Host = host

	// Because we changed the host, we need to resign the request to the new host
	canonicalRequest := getCanonicalRequest(r.Request)
	stringToSign := getStringToSign(r.Request, canonicalRequest)

	// TODO look up key secret from ID
	keySecret := ""

	signingKey := getSigningKey(r.Request, keySecret)
	signature := fmt.Sprintf("%x", getHMAC(signingKey, []byte(stringToSign)))

	// Update the auth header with the new signature
	originalAuthHeader := r.Request.Header.Get("Authorization")
	re := regexp.MustCompile(`Signature=[^,]+`)
	re.ReplaceAllString(originalAuthHeader, "Signature="+signature)

	// Now we can do the original request
	req, err := http.NewRequestWithContext(ctx, r.Request.Method, originalURL.String(), r.Request.Body)
	if err != nil {
		return nil, fmt.Errorf("error in http.NewRequestWithContext: %w", err)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error in http.DefaultClient.Do: %w", err)
	}

	return res, nil
}

type AWSService interface {
	// HandleRequest will handle a request for a given service.
	// Returning a *http.Response will stream that response to the client
	HandleRequest(ctx context.Context, request *ProxiedRequest) (*http.Response, error)
}
