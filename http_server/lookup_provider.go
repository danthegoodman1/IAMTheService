package http_server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

// LookupProvider is used to perform KV lookups for things like
// AWS Key ID to key secret and incoming host to destination host
type LookupProvider[TKey, TVal any] interface {
	Lookup(ctx context.Context, key TKey) (*TVal, error)
}

// EnvJSONLookupProvider uses an env var encoded as JSON like HOSTS={"key":"val"}
// where keys and values are wrapped in single quotes, separated by colons,
// and pairs separated by commas
type EnvJSONLookupProvider struct {
	m map[string]string
}

// NewEnvJSONLookupProvider will create a new EnvJSONLookupProvider from a given env var.
func NewEnvJSONLookupProvider(envVar string) (EnvJSONLookupProvider, error) {
	envMap := map[string]string{}

	err := json.Unmarshal([]byte(os.Getenv(envVar)), &envMap)
	if err != nil {
		return EnvJSONLookupProvider{}, fmt.Errorf("error in json.Marshal for %s: %w", envVar, err)
	}

	return EnvJSONLookupProvider{m: envMap}, nil
}

func (e EnvJSONLookupProvider) Lookup(_ context.Context, key string) (*string, error) {
	if val, exists := e.m[key]; exists {
		return &val, nil
	}

	return nil, nil
}
