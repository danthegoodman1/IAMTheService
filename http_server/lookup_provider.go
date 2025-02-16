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
	Lookup(ctx context.Context, key TKey) (TVal, error)
}

// EnvJSONLookupProvider uses an env var encoded as JSON like HOST_MAP={"key":"val"}
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

func (e EnvJSONLookupProvider) Lookup(_ context.Context, key string) (string, error) {
	if val, exists := e.m[key]; exists {
		return val, nil
	}

	return "", nil
}

// MapLookupProvider just uses a map
type MapLookupProvider[TKey comparable, TVal any] struct {
	m map[TKey]TVal
}

// NewMapLookupProvider will create a new MapLookupProvider from a given env var.
func NewMapLookupProvider[TKey comparable, TVal any](m map[TKey]TVal) (MapLookupProvider[TKey, TVal], error) {
	return MapLookupProvider[TKey, TVal]{m}, nil
}

func (e MapLookupProvider[TKey, TVal]) Lookup(_ context.Context, key TKey) (TVal, error) {
	if val, exists := e.m[key]; exists {
		return val, nil
	}
	var defVal TVal
	return defVal, nil
}
