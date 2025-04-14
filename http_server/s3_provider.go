package http_server

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// S3OperationHandler is a function type for handling specific S3 operations
type S3OperationHandler func(ctx context.Context, request *ProxiedRequest) (*http.Response, error)

// S3Provider handles S3-specific AWS requests
type S3Provider struct {
	*BaseAWSProvider
	operationHandlers map[string]S3OperationHandler
	pathPatterns      map[string]*regexp.Regexp
	targetHost        string
}

// NewS3Provider creates a new S3 service provider
func NewS3Provider() *S3Provider {
	provider := &S3Provider{
		BaseAWSProvider:   NewBaseAWSProvider("s3"),
		operationHandlers: make(map[string]S3OperationHandler),
		pathPatterns:      make(map[string]*regexp.Regexp),
		targetHost:        "s3.amazonaws.com",
	}

	// Initialize with default path patterns for common S3 operations
	provider.pathPatterns["GetObject"] = regexp.MustCompile(`^/[^/]+/.*[^/]$`)
	provider.pathPatterns["PutObject"] = regexp.MustCompile(`^/[^/]+/.*[^/]$`)
	provider.pathPatterns["DeleteObject"] = regexp.MustCompile(`^/[^/]+/.*[^/]$`)
	provider.pathPatterns["ListObjects"] = regexp.MustCompile(`^/[^/]+/?(\?.+)?$`)
	provider.pathPatterns["CreateBucket"] = regexp.MustCompile(`^/[^/]+/?$`)
	provider.pathPatterns["DeleteBucket"] = regexp.MustCompile(`^/[^/]+/?$`)

	return provider
}

// RegisterOperationHandler registers a handler for a specific S3 operation
func (p *S3Provider) RegisterOperationHandler(operation string, handler S3OperationHandler) {
	p.operationHandlers[operation] = handler
}

// ExtractOperationName determines the S3 operation from the request
func (p *S3Provider) ExtractOperationName(request *ProxiedRequest) string {
	path := request.Request.URL.Path
	method := request.Request.Method

	// Check for specific query parameters that indicate certain operations
	query := request.Request.URL.Query()

	// Special case handling for operations identified by query parameters
	if len(query) > 0 {
		if _, ok := query["list-type"]; ok {
			return "ListObjectsV2"
		}
		if _, ok := query["uploads"]; ok {
			if method == "POST" {
				return "CreateMultipartUpload"
			}
			return "ListMultipartUploads"
		}
		// Add more special cases as needed
	}

	// Match based on HTTP method and path pattern
	for operation, pattern := range p.pathPatterns {
		if pattern.MatchString(path) {
			switch method {
			case "GET":
				if operation == "GetObject" || operation == "ListObjects" {
					if strings.HasSuffix(path, "/") || path == "/" {
						return "ListObjects"
					}
					return "GetObject"
				}
			case "PUT":
				if operation == "PutObject" || operation == "CreateBucket" {
					if strings.Count(path, "/") <= 1 {
						return "CreateBucket"
					}
					return "PutObject"
				}
			case "DELETE":
				if operation == "DeleteObject" || operation == "DeleteBucket" {
					if strings.Count(path, "/") <= 1 {
						return "DeleteBucket"
					}
					return "DeleteObject"
				}
			}
		}
	}

	return "Unknown"
}

// HandleRequest processes S3 requests, using registered handlers when available
func (p *S3Provider) HandleRequest(ctx context.Context, request *ProxiedRequest) (*http.Response, error) {
	// Determine which S3 operation is being requested
	operation := p.ExtractOperationName(request)

	// Check if we have a registered handler for this operation
	if handler, exists := p.operationHandlers[operation]; exists {
		return handler(ctx, request)
	}

	// If no handler is registered, use the default proxying behavior
	return p.BaseAWSProvider.HandleRequest(ctx, request)
}

// ParseS3Request extracts bucket and key information from an S3 request
func (p *S3Provider) ParseS3Request(request *ProxiedRequest) (bucket, key string, err error) {
	path := request.Request.URL.Path

	// Remove leading slash
	if strings.HasPrefix(path, "/") {
		path = path[1:]
	}

	// Split path into segments
	segments := strings.SplitN(path, "/", 2)

	if len(segments) == 0 {
		return "", "", fmt.Errorf("invalid S3 path: %s", path)
	}

	bucket = segments[0]

	if len(segments) > 1 {
		key = segments[1]
	}

	return bucket, key, nil
}
