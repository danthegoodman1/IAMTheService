package http_server

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// ExampleUsage demonstrates how to set up and use the AWS service provider framework
func ExampleUsage() {
	// Create a registry for AWS service providers
	registry := NewAWSServiceRegistry()

	// Create an S3 provider
	s3Provider := NewS3Provider()

	// Register custom handlers for specific S3 operations

	// Example 1: Cache GetObject responses
	s3Provider.RegisterOperationHandler("GetObject", func(ctx context.Context, request *ProxiedRequest) (*http.Response, error) {
		bucket, key, err := s3Provider.ParseS3Request(request)
		if err != nil {
			return nil, err
		}

		cacheKey := fmt.Sprintf("s3:%s:%s", bucket, key)

		// Check if object is in cache (simplified example)
		cachedObject := checkCache(cacheKey)
		if cachedObject != nil {
			// Return cached object
			return createSuccessResponse(cachedObject), nil
		}

		// Not in cache, proxy to origin
		resp, err := request.DoProxiedRequest(ctx, "s3.amazonaws.com")
		if err != nil {
			return nil, err
		}

		// Store in cache for future requests (simplified)
		if resp.StatusCode == http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			// Store in cache
			storeInCache(cacheKey, body)

			// Create a new response with the same body
			resp.Body = io.NopCloser(bytes.NewReader(body))
		}

		return resp, nil
	})

	// Example 2: Multi-tenant S3 path rewriting
	s3Provider.RegisterOperationHandler("PutObject", func(ctx context.Context, request *ProxiedRequest) (*http.Response, error) {
		bucket, key, err := s3Provider.ParseS3Request(request)
		if err != nil {
			return nil, err
		}

		// Extract tenant ID from request context or headers
		tenantID := extractTenantID(request)

		// Rewrite the path to include tenant prefix
		newKey := fmt.Sprintf("%s/%s", tenantID, key)

		// Create a modified request with the new path
		modifiedRequest := cloneRequest(request)
		modifiedRequest.Request.URL.Path = fmt.Sprintf("/%s/%s", bucket, newKey)

		// Proxy the modified request
		return modifiedRequest.DoProxiedRequest(ctx, "s3.amazonaws.com")
	})

	// Register the S3 provider with the registry
	registry.RegisterProvider(s3Provider)

	// In your HTTP handler, you would use the registry to find and use the appropriate provider
	// httpHandler := func(w http.ResponseWriter, r *http.Request) {
	//     proxiedRequest := createProxiedRequest(r)
	//     provider, found := registry.GetProviderForRequest(proxiedRequest)
	//     if !found {
	//         // No provider found, handle accordingly
	//         http.Error(w, "Service not supported", http.StatusBadGateway)
	//         return
	//     }
	//
	//     resp, err := provider.HandleRequest(r.Context(), proxiedRequest)
	//     if err != nil {
	//         http.Error(w, err.Error(), http.StatusInternalServerError)
	//         return
	//     }
	//
	//     // Copy response to the client
	//     copyResponse(w, resp)
	// }
}

// Helper functions (these would be implemented elsewhere in a real application)

func checkCache(key string) []byte {
	// This is a placeholder for a real cache implementation
	return nil
}

func storeInCache(key string, data []byte) {
	// This is a placeholder for a real cache implementation
}

func extractTenantID(request *ProxiedRequest) string {
	// This is a placeholder for extracting tenant ID from request
	// Could come from a header, JWT token, etc.
	return "tenant-123"
}

func cloneRequest(original *ProxiedRequest) *ProxiedRequest {
	// This is a simplified clone function
	// In a real implementation, you'd need to properly clone the request
	return original
}

func createSuccessResponse(data []byte) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(data)),
		Header:     make(http.Header),
	}
}

func copyResponse(w http.ResponseWriter, resp *http.Response) {
	// Copy headers
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// Set status code
	w.WriteHeader(resp.StatusCode)

	// Copy body
	io.Copy(w, resp.Body)
	resp.Body.Close()
}
